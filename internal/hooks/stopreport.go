package hooks

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/sqltx"
)

const (
	watermarkPrefix = "stop_report:"
	legacyMetaKey   = "last_stop_report"
	fallbackWindow  = 30 * time.Minute
	watermarkTTL    = 7 * 24 * time.Hour
	maxStdinBytes   = 1 << 20
	sqliteTimeFmt   = "2006-01-02 15:04:05"
)

// Both queries filter on datetime(created_at) so that RFC3339 rows (written
// by LogActivity) and SQLite-default "YYYY-MM-DD HH:MM:SS" rows (web UI)
// compare at the same second precision. The expression must stay textually
// identical to the one in idx_activity_log_datetime or the planner falls
// back to a full scan.
const (
	sameSecondActivityQuery = `SELECT id FROM activity_log WHERE datetime(created_at) = datetime(?)`
	activitySinceQuery      = `
SELECT id, board_id, card_id, action, details, agent, created_at, datetime(created_at) AS bucket
FROM activity_log
WHERE datetime(created_at) >= datetime(?)
ORDER BY bucket DESC, id`
)

// watermark is stored per session in `_meta` under stop_report:<session_id>.
// `_meta` is shared with the web UI, so only keys with that prefix are touched here.
type watermark struct {
	Since     string   `json:"since"`
	SeenIDs   []string `json:"seen_ids"`
	UpdatedAt string   `json:"updated_at"`
}

func watermarkKey(sessionID string) string { return watermarkPrefix + sessionID }

// ReadSessionID extracts session_id from the hook JSON Claude Code writes to stdin.
// Any read or decode failure yields "" so the hooks keep their exit-0 contract.
func ReadSessionID(r io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(r, maxStdinBytes))
	if err != nil {
		return ""
	}
	var payload struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return payload.SessionID
}

// RecordSessionStart pins the session's watermark to now so its first Stop
// reports only activity logged during the session.
func RecordSessionStart(db *sqlx.DB, sessionID string, now time.Time) error {
	if sessionID == "" {
		return nil
	}
	since := now.UTC().Format(sqliteTimeFmt)
	var seen []string
	if err := db.Select(&seen, sameSecondActivityQuery, since); err != nil {
		return fmt.Errorf("load same-second activity: %w", err)
	}
	return saveWatermark(db, sessionID, watermark{Since: since, SeenIDs: seen}, now)
}

// StopReport returns the plain-text activity summary since the session's
// watermark ("" when nothing is new) and advances that watermark.
func StopReport(db *sqlx.DB, sessionID string, now time.Time) (string, error) {
	if err := pruneWatermarks(db, now); err != nil {
		return "", err
	}

	wm, found, err := loadWatermark(db, sessionID)
	if err != nil {
		return "", err
	}
	if !found {
		wm = watermark{Since: now.Add(-fallbackWindow).UTC().Format(sqliteTimeFmt)}
	}

	rows, err := activitySince(db, wm.Since)
	if err != nil {
		return "", err
	}
	fresh := dropSeen(rows, wm.SeenIDs)

	if len(fresh) > 0 {
		// Every row in the newest second stays in seen_ids, including ones
		// reported by an earlier stop, so a later stop never repeats them.
		wm.Since = fresh[0].Bucket
		wm.SeenIDs = nil
		for _, r := range rows {
			if r.Bucket == wm.Since {
				wm.SeenIDs = append(wm.SeenIDs, r.ID)
			}
		}
	}
	if sessionID != "" {
		if err := saveWatermark(db, sessionID, wm, now); err != nil {
			return "", err
		}
	}

	logs := make([]models.ActivityLog, 0, len(fresh))
	for _, r := range fresh {
		logs = append(logs, r.ActivityLog)
	}
	return FormatSummary(logs), nil
}

type activityRow struct {
	models.ActivityLog
	Bucket string `db:"bucket"`
}

func activitySince(db *sqlx.DB, since string) ([]activityRow, error) {
	var rows []activityRow
	err := db.Select(&rows, activitySinceQuery, since)
	if err != nil {
		return nil, fmt.Errorf("load activity: %w", err)
	}
	return rows, nil
}

func dropSeen(rows []activityRow, seen []string) []activityRow {
	if len(seen) == 0 {
		return rows
	}
	skip := make(map[string]struct{}, len(seen))
	for _, id := range seen {
		skip[id] = struct{}{}
	}
	out := make([]activityRow, 0, len(rows))
	for _, r := range rows {
		if _, ok := skip[r.ID]; !ok {
			out = append(out, r)
		}
	}
	return out
}

func loadWatermark(db *sqlx.DB, sessionID string) (watermark, bool, error) {
	if sessionID == "" {
		return watermark{}, false, nil
	}
	var raw string
	err := db.Get(&raw, `SELECT value FROM _meta WHERE key = ?`, watermarkKey(sessionID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return watermark{}, false, nil
		}
		return watermark{}, false, fmt.Errorf("load watermark: %w", err)
	}
	var wm watermark
	if err := json.Unmarshal([]byte(raw), &wm); err != nil || wm.Since == "" {
		return watermark{}, false, nil
	}
	return wm, true, nil
}

func saveWatermark(db *sqlx.DB, sessionID string, wm watermark, now time.Time) error {
	wm.UpdatedAt = now.UTC().Format(time.RFC3339)
	if wm.SeenIDs == nil {
		wm.SeenIDs = []string{}
	}
	data, err := json.Marshal(wm)
	if err != nil {
		return fmt.Errorf("encode watermark: %w", err)
	}
	if _, err := db.Exec(`INSERT OR REPLACE INTO _meta (key, value) VALUES (?, ?)`, watermarkKey(sessionID), string(data)); err != nil {
		return fmt.Errorf("save watermark: %w", err)
	}
	return nil
}

// pruneWatermarks drops session watermarks untouched for watermarkTTL (and
// unparsable ones) plus the pre-H8 global key, so `_meta` stays bounded.
func pruneWatermarks(db *sqlx.DB, now time.Time) error {
	type entry struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}
	var entries []entry
	if err := db.Select(&entries, `SELECT key, value FROM _meta WHERE key LIKE ?`, watermarkPrefix+"%"); err != nil {
		return fmt.Errorf("list watermarks: %w", err)
	}

	cutoff := now.Add(-watermarkTTL)
	stale := []string{legacyMetaKey}
	for _, e := range entries {
		var wm watermark
		updated, parseErr := time.Time{}, json.Unmarshal([]byte(e.Value), &wm)
		if parseErr == nil {
			updated, parseErr = time.Parse(time.RFC3339, wm.UpdatedAt)
		}
		if parseErr != nil || updated.Before(cutoff) {
			stale = append(stale, e.Key)
		}
	}

	return sqltx.Run(db, func(tx *sqlx.Tx) error {
		for _, key := range stale {
			if _, err := tx.Exec(`DELETE FROM _meta WHERE key = ?`, key); err != nil {
				return fmt.Errorf("prune watermark %s: %w", key, err)
			}
		}
		return nil
	})
}

// FormatSummary renders per-action counts as plain text: Claude Code does not
// render markdown in hook output.
func FormatSummary(activities []models.ActivityLog) string {
	counts := map[string]int{}
	for _, a := range activities {
		counts[a.Action]++
	}

	actionLabels := []struct{ key, label string }{
		{"card_created", "Cards created"},
		{"card_updated", "Cards updated"},
		{"card_moved", "Cards moved"},
		{"card_deleted", "Cards deleted"},
		{"column_created", "Columns created"},
		{"phase_created", "Phases created"},
		{"phase_updated", "Phases updated"},
		{"phase_deleted", "Phases deleted"},
		{"phases_reordered", "Phases reordered"},
		{"tag_created", "Tags created"},
		{"tag_assigned", "Tags assigned"},
		{"tag_removed", "Tags removed"},
		{"tag_deleted", "Tags deleted"},
	}
	var sb strings.Builder
	for _, al := range actionLabels {
		if n := counts[al.key]; n > 0 {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", al.label, n))
		}
	}
	return sb.String()
}
