package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/sqltx"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

// ResolvePhase returns nil, nil when neither phaseID nor phaseName is given.
func ResolvePhase(db Querier, boardID, phaseID, phaseName string) (*models.Phase, error) {
	if phaseID == "" && phaseName == "" {
		return nil, nil
	}

	if phaseID != "" {
		return getPhase(db, phaseID)
	}

	var phase models.Phase
	err := db.Get(&phase, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? AND name = ? COLLATE NOCASE`, boardID, phaseName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("NOT_FOUND: phase %q not found", phaseName)
	}
	if err != nil {
		return nil, fmt.Errorf("get phase: %w", err)
	}
	return &phase, nil
}

func getPhase(q Querier, phaseID string) (*models.Phase, error) {
	var phase models.Phase
	err := q.Get(&phase, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE id = ?`, phaseID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("NOT_FOUND: phase not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get phase: %w", err)
	}
	return &phase, nil
}

func ManagePhases(db *sqlx.DB, action, boardID, phaseID, name, color string, orderedIDs []string) (interface{}, error) {
	switch action {
	case "list":
		return listPhases(db, boardID)
	case "create":
		return createPhase(db, boardID, name, color)
	case "update":
		return updatePhase(db, phaseID, name, color)
	case "delete":
		return deletePhase(db, phaseID)
	case "reorder":
		return reorderPhases(db, boardID, orderedIDs)
	default:
		return nil, fmt.Errorf("VALIDATION: unknown action %q", action)
	}
}

// ParseOrderedIDs accepts a JSON array or a comma-separated list.
func ParseOrderedIDs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}

	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			return nil, fmt.Errorf("VALIDATION: invalid ordered_ids JSON: %w", err)
		}
		return ids, nil
	}

	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ids = append(ids, p)
		}
	}
	return ids, nil
}

func listPhases(db *sqlx.DB, boardID string) ([]models.Phase, error) {
	if boardID == "" {
		board, err := ResolveBoard(db, "")
		if err != nil {
			return nil, err
		}
		boardID = board.ID
	}
	var phases []models.Phase
	if err := db.Select(&phases, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, boardID); err != nil {
		return nil, err
	}
	if phases == nil {
		phases = []models.Phase{}
	}
	return phases, nil
}

func createPhase(db *sqlx.DB, boardID, name, color string) (*models.Phase, error) {
	if boardID == "" {
		board, err := ResolveBoard(db, "")
		if err != nil {
			return nil, err
		}
		boardID = board.ID
	}

	if name == "" {
		return nil, fmt.Errorf("VALIDATION: name is required")
	}
	if len(name) > 50 {
		return nil, fmt.Errorf("VALIDATION: name must be 50 characters or less")
	}

	if color == "" {
		color = "#00FFFF"
	}
	if !strings.HasPrefix(color, "#") || len(color) != 7 {
		return nil, fmt.Errorf("VALIDATION: color must be a 7-character hex color (e.g. #00FFFF)")
	}

	var boardExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM boards WHERE id = ?`, boardID).Scan(&boardExists); err != nil {
		return nil, fmt.Errorf("check board: %w", err)
	}
	if boardExists == 0 {
		return nil, fmt.Errorf("NOT_FOUND: board not found")
	}

	var existing int
	if err := db.QueryRow(`SELECT COUNT(*) FROM phases WHERE board_id = ? AND name = ? COLLATE NOCASE`, boardID, name).Scan(&existing); err != nil {
		return nil, fmt.Errorf("check phase name: %w", err)
	}
	if existing > 0 {
		return nil, fmt.Errorf("CONFLICT: phase %q already exists on this board", name)
	}

	var maxPos float64
	db.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM phases WHERE board_id = ?`, boardID).Scan(&maxPos)
	position := maxPos + 1.0

	id, _ := gonanoid.New(12)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO phases (id, board_id, name, color, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, boardID, name, color, position, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert phase: %w", err)
	}

	if err := LogActivity(db, boardID, nil, "phase_created", fmt.Sprintf("Created phase: %s", name), ""); err != nil {
		return nil, fmt.Errorf("log activity: %w", err)
	}

	return &models.Phase{ID: id, BoardID: boardID, Name: name, Color: color, Position: position, CreatedAt: now, UpdatedAt: now}, nil
}

func updatePhase(db *sqlx.DB, phaseID, name, color string) (*models.Phase, error) {
	if phaseID == "" {
		return nil, fmt.Errorf("VALIDATION: phase_id is required")
	}

	phase, err := getPhase(db, phaseID)
	if err != nil {
		return nil, err
	}

	if name != "" {
		if len(name) > 50 {
			return nil, fmt.Errorf("VALIDATION: name must be 50 characters or less")
		}
		var existing int
		if err := db.QueryRow(`SELECT COUNT(*) FROM phases WHERE board_id = ? AND name = ? COLLATE NOCASE AND id != ?`, phase.BoardID, name, phaseID).Scan(&existing); err != nil {
			return nil, fmt.Errorf("check phase name: %w", err)
		}
		if existing > 0 {
			return nil, fmt.Errorf("CONFLICT: phase %q already exists on this board", name)
		}
		phase.Name = name
	}

	if color != "" {
		if !strings.HasPrefix(color, "#") || len(color) != 7 {
			return nil, fmt.Errorf("VALIDATION: color must be a 7-character hex color (e.g. #00FFFF)")
		}
		phase.Color = color
	}

	phase.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err = db.Exec(
		`UPDATE phases SET name = ?, color = ?, updated_at = ? WHERE id = ?`,
		phase.Name, phase.Color, phase.UpdatedAt, phase.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update phase: %w", err)
	}

	if err := LogActivity(db, phase.BoardID, nil, "phase_updated", fmt.Sprintf("Updated phase: %s", phase.Name), ""); err != nil {
		return nil, fmt.Errorf("log activity: %w", err)
	}

	return phase, nil
}

func deletePhase(db *sqlx.DB, phaseID string) (map[string]interface{}, error) {
	if phaseID == "" {
		return nil, fmt.Errorf("VALIDATION: phase_id is required")
	}

	phase, err := getPhase(db, phaseID)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`DELETE FROM phases WHERE id = ?`, phaseID); err != nil {
		return nil, fmt.Errorf("delete phase: %w", err)
	}

	if err := LogActivity(db, phase.BoardID, nil, "phase_deleted", fmt.Sprintf("Deleted phase: %s", phase.Name), ""); err != nil {
		return nil, fmt.Errorf("log activity: %w", err)
	}

	return map[string]interface{}{"deleted": true, "phase_id": phaseID}, nil
}

func reorderPhases(db *sqlx.DB, boardID string, orderedIDs []string) ([]models.Phase, error) {
	var result []models.Phase
	err := sqltx.Run(db, func(tx *sqlx.Tx) error {
		var err error
		result, err = reorderPhasesTx(tx, boardID, orderedIDs)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func reorderPhasesTx(tx *sqlx.Tx, boardID string, orderedIDs []string) ([]models.Phase, error) {
	if boardID == "" {
		board, err := ResolveBoard(tx, "")
		if err != nil {
			return nil, err
		}
		boardID = board.ID
	}

	if len(orderedIDs) == 0 {
		return nil, fmt.Errorf("VALIDATION: ordered_ids is required for reorder")
	}

	var phases []models.Phase
	if err := tx.Select(&phases, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, boardID); err != nil {
		return nil, err
	}

	if len(orderedIDs) != len(phases) {
		return nil, fmt.Errorf("VALIDATION: ordered_ids count (%d) does not match phase count (%d)", len(orderedIDs), len(phases))
	}

	phaseMap := make(map[string]bool, len(phases))
	for _, p := range phases {
		phaseMap[p.ID] = true
	}

	seen := make(map[string]bool, len(orderedIDs))
	for _, id := range orderedIDs {
		if !phaseMap[id] {
			return nil, fmt.Errorf("VALIDATION: phase %q does not belong to this board", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("VALIDATION: duplicate phase ID %q in ordered_ids", id)
		}
		seen[id] = true
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for i, id := range orderedIDs {
		pos := float64(i + 1)
		if _, err := tx.Exec(`UPDATE phases SET position = ?, updated_at = ? WHERE id = ?`, pos, now, id); err != nil {
			return nil, fmt.Errorf("reorder phase: %w", err)
		}
	}

	var result []models.Phase
	if err := tx.Select(&result, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, boardID); err != nil {
		return nil, err
	}
	if result == nil {
		result = []models.Phase{}
	}

	if err := LogActivity(tx, boardID, nil, "phases_reordered", "Phases reordered", ""); err != nil {
		return nil, fmt.Errorf("log activity: %w", err)
	}

	return result, nil
}
