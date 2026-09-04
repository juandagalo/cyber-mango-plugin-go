package hooks

import (
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	testDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	if err := db.RunMigrations(testDB); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := db.SeedDefaultBoard(testDB); err != nil {
		t.Fatalf("seed default board: %v", err)
	}
	return testDB
}

func firstBoardID(t *testing.T, testDB *sqlx.DB) string {
	t.Helper()
	var id string
	if err := testDB.Get(&id, `SELECT id FROM boards ORDER BY created_at LIMIT 1`); err != nil {
		t.Fatalf("first board: %v", err)
	}
	return id
}

func insertActivity(t *testing.T, testDB *sqlx.DB, id, action, createdAt string) {
	t.Helper()
	_, err := testDB.Exec(
		`INSERT INTO activity_log (id, board_id, action, created_at) VALUES (?, ?, ?, ?)`,
		id, firstBoardID(t, testDB), action, createdAt,
	)
	if err != nil {
		t.Fatalf("insert activity %s: %v", id, err)
	}
}

func rfc3339(tm time.Time) string { return tm.UTC().Format(time.RFC3339) }

func sqliteFormat(tm time.Time) string { return tm.UTC().Format("2006-01-02 15:04:05") }

func stop(t *testing.T, testDB *sqlx.DB, sessionID string, now time.Time) string {
	t.Helper()
	out, err := StopReport(testDB, sessionID, now)
	if err != nil {
		t.Fatalf("stop report for %q: %v", sessionID, err)
	}
	return out
}

func mustContain(t *testing.T, out, want string) {
	t.Helper()
	if !strings.Contains(out, want) {
		t.Fatalf("expected output to contain %q, got %q", want, out)
	}
}

func TestConcurrentSessionsKeepSeparateWatermarks(t *testing.T) {
	testDB := newTestDB(t)
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

	if err := RecordSessionStart(testDB, "A", base); err != nil {
		t.Fatal(err)
	}
	insertActivity(t, testDB, "beforeB", "card_created", rfc3339(base.Add(1*time.Minute)))

	if err := RecordSessionStart(testDB, "B", base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	insertActivity(t, testDB, "afterB", "card_moved", rfc3339(base.Add(3*time.Minute)))

	outA := stop(t, testDB, "A", base.Add(4*time.Minute))
	mustContain(t, outA, "Cards created: 1")
	mustContain(t, outA, "Cards moved: 1")

	outB := stop(t, testDB, "B", base.Add(4*time.Minute))
	if strings.Contains(outB, "Cards created") {
		t.Fatalf("session B must not report activity logged before it started, got %q", outB)
	}
	mustContain(t, outB, "Cards moved: 1")

	if again := stop(t, testDB, "A", base.Add(5*time.Minute)); again != "" {
		t.Fatalf("second stop of A with no new activity must be empty, got %q", again)
	}
}

func TestSameSecondActivityIsNotDroppedAndNotRepeated(t *testing.T) {
	testDB := newTestDB(t)
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	if err := RecordSessionStart(testDB, "S", base); err != nil {
		t.Fatal(err)
	}
	at := rfc3339(base.Add(30 * time.Second))

	insertActivity(t, testDB, "first", "card_created", at)
	out := stop(t, testDB, "S", base.Add(time.Minute))
	mustContain(t, out, "Cards created: 1")

	insertActivity(t, testDB, "second", "card_created", at)
	out = stop(t, testDB, "S", base.Add(2*time.Minute))
	mustContain(t, out, "Cards created: 1")

	if again := stop(t, testDB, "S", base.Add(3*time.Minute)); again != "" {
		t.Fatalf("nothing new after same-second rows were reported, got %q", again)
	}
}

func TestMixedTimestampFormatsCompareByDatetime(t *testing.T) {
	testDB := newTestDB(t)
	base := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)
	if err := RecordSessionStart(testDB, "S", base); err != nil {
		t.Fatal(err)
	}

	insertActivity(t, testDB, "oldRFC", "card_created", rfc3339(base.Add(-time.Minute)))
	insertActivity(t, testDB, "oldSQL", "card_created", sqliteFormat(base.Add(-time.Minute)))
	insertActivity(t, testDB, "newRFC", "card_moved", rfc3339(base.Add(time.Minute)))
	insertActivity(t, testDB, "newSQL", "card_deleted", sqliteFormat(base.Add(2*time.Minute)))

	out := stop(t, testDB, "S", base.Add(3*time.Minute))
	if strings.Contains(out, "Cards created") {
		t.Fatalf("rows older than the watermark must be excluded in both formats, got %q", out)
	}
	mustContain(t, out, "Cards moved: 1")
	mustContain(t, out, "Cards deleted: 1")
}

func TestNoSessionIDFallsBackToLastThirtyMinutes(t *testing.T) {
	testDB := newTestDB(t)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	insertActivity(t, testDB, "old", "card_created", rfc3339(now.Add(-2*time.Hour)))
	insertActivity(t, testDB, "recent", "card_moved", rfc3339(now.Add(-5*time.Minute)))

	out := stop(t, testDB, "", now)
	if strings.Contains(out, "Cards created") {
		t.Fatalf("activity older than 30 minutes must not be reported, got %q", out)
	}
	mustContain(t, out, "Cards moved: 1")

	var n int
	if err := testDB.Get(&n, `SELECT COUNT(*) FROM _meta WHERE key LIKE 'stop_report:%'`); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("no watermark must be written without a session id, found %d", n)
	}
}

func TestUnknownSessionFallsBackAndWritesWatermark(t *testing.T) {
	testDB := newTestDB(t)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	insertActivity(t, testDB, "old", "card_created", rfc3339(now.Add(-2*time.Hour)))
	insertActivity(t, testDB, "recent", "card_moved", rfc3339(now.Add(-5*time.Minute)))

	out := stop(t, testDB, "fresh", now)
	if strings.Contains(out, "Cards created") {
		t.Fatalf("activity older than 30 minutes must not be reported, got %q", out)
	}
	mustContain(t, out, "Cards moved: 1")

	var value string
	if err := testDB.Get(&value, `SELECT value FROM _meta WHERE key = 'stop_report:fresh'`); err != nil {
		t.Fatalf("watermark for session must exist: %v", err)
	}
	if again := stop(t, testDB, "fresh", now.Add(time.Minute)); again != "" {
		t.Fatalf("second stop must not repeat the reported activity, got %q", again)
	}
}

func TestLegacyGlobalWatermarkIsIgnoredAndRemoved(t *testing.T) {
	testDB := newTestDB(t)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	if _, err := testDB.Exec(`INSERT INTO _meta (key, value) VALUES ('last_stop_report', ?)`, rfc3339(now)); err != nil {
		t.Fatal(err)
	}
	insertActivity(t, testDB, "recent", "card_moved", rfc3339(now.Add(-5*time.Minute)))

	mustContain(t, stop(t, testDB, "", now), "Cards moved: 1")

	var n int
	if err := testDB.Get(&n, `SELECT COUNT(*) FROM _meta WHERE key = 'last_stop_report'`); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("legacy last_stop_report key must be deleted")
	}
}

func TestStopPrunesStaleWatermarks(t *testing.T) {
	testDB := newTestDB(t)
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	if err := RecordSessionStart(testDB, "old", now.Add(-8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := RecordSessionStart(testDB, "young", now.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := testDB.Exec(`INSERT INTO _meta (key, value) VALUES ('stop_report:broken', 'not json')`); err != nil {
		t.Fatal(err)
	}

	stop(t, testDB, "", now)

	var keys []string
	if err := testDB.Select(&keys, `SELECT key FROM _meta ORDER BY key`); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(keys, ",")
	for _, want := range []string{"schema_version", "stop_report:young"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %s to survive pruning, keys: %s", want, got)
		}
	}
	for _, gone := range []string{"stop_report:old", "stop_report:broken"} {
		if strings.Contains(got, gone) {
			t.Fatalf("expected %s to be pruned, keys: %s", gone, got)
		}
	}
}

func TestFormatSummaryCountsEveryAction(t *testing.T) {
	actions := []string{
		"card_created", "card_created", "card_updated", "card_moved", "card_deleted",
		"column_created", "phase_created", "phase_updated", "phase_deleted", "phases_reordered",
		"tag_created", "tag_assigned", "tag_assigned", "tag_removed", "tag_deleted",
	}
	var logs []models.ActivityLog
	for _, a := range actions {
		logs = append(logs, models.ActivityLog{Action: a})
	}

	out := FormatSummary(logs)
	want := "  Cards created: 2\n" +
		"  Cards updated: 1\n" +
		"  Cards moved: 1\n" +
		"  Cards deleted: 1\n" +
		"  Columns created: 1\n" +
		"  Phases created: 1\n" +
		"  Phases updated: 1\n" +
		"  Phases deleted: 1\n" +
		"  Phases reordered: 1\n" +
		"  Tags created: 1\n" +
		"  Tags assigned: 2\n" +
		"  Tags removed: 1\n" +
		"  Tags deleted: 1\n"
	if out != want {
		t.Fatalf("unexpected summary:\n%s\nwant:\n%s", out, want)
	}
	if strings.ContainsAny(out, "*#") {
		t.Fatal("summary must be plain text without markdown")
	}
}

func TestFormatSummaryEmptyWhenNothingNew(t *testing.T) {
	if out := FormatSummary(nil); out != "" {
		t.Fatalf("expected empty summary, got %q", out)
	}
}

func TestReadSessionID(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"valid", `{"session_id":"abc-123","hook_event_name":"Stop"}`, "abc-123"},
		{"empty", ``, ""},
		{"invalid", `{not json`, ""},
		{"missing field", `{"hook_event_name":"Stop"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReadSessionID(strings.NewReader(tc.in)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func queryPlan(t *testing.T, testDB *sqlx.DB, query string, args ...interface{}) string {
	t.Helper()
	rows, err := testDB.Queryx("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		plan = append(plan, detail)
	}
	return strings.Join(plan, "\n")
}

// The hooks filter on datetime(created_at), which a plain index on created_at
// cannot serve; this pins the expression index to the exact query text.
func TestActivityQueriesUseDatetimeExpressionIndex(t *testing.T) {
	testDB := newTestDB(t)
	for name, query := range map[string]string{
		"activitySince": activitySinceQuery,
		"sameSecond":    sameSecondActivityQuery,
	} {
		t.Run(name, func(t *testing.T) {
			plan := queryPlan(t, testDB, query, "2026-09-03 10:00:00")
			if !strings.Contains(plan, "USING INDEX idx_activity_log_datetime") {
				t.Fatalf("expected idx_activity_log_datetime in plan, got:\n%s", plan)
			}
		})
	}
}
