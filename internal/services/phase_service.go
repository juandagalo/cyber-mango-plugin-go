package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

// ResolvePhase resolves a phase by ID or name (case-insensitive) within a board.
// Returns nil, nil if both phaseID and phaseName are empty (no phase requested).
func ResolvePhase(db *sqlx.DB, boardID, phaseID, phaseName string) (*models.Phase, error) {
	if phaseID == "" && phaseName == "" {
		return nil, nil
	}

	var phase models.Phase

	if phaseID != "" {
		if err := db.Get(&phase, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE id = ?`, phaseID); err != nil {
			return nil, fmt.Errorf("NOT_FOUND: phase not found")
		}
		return &phase, nil
	}

	if err := db.Get(&phase, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? AND LOWER(name) = LOWER(?)`, boardID, phaseName); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: phase %q not found", phaseName)
	}
	return &phase, nil
}

// ManagePhases dispatches to the appropriate phase operation.
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

// ParseOrderedIDs parses ordered_ids from a JSON array string or comma-separated string.
func ParseOrderedIDs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}

	// Try JSON array first
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			return nil, fmt.Errorf("VALIDATION: invalid ordered_ids JSON: %w", err)
		}
		return ids, nil
	}

	// Fallback: comma-separated
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

	// Check board exists
	var boardExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM boards WHERE id = ?`, boardID).Scan(&boardExists); err != nil || boardExists == 0 {
		return nil, fmt.Errorf("NOT_FOUND: board not found")
	}

	// Check name uniqueness on board
	var existing int
	db.QueryRow(`SELECT COUNT(*) FROM phases WHERE board_id = ? AND LOWER(name) = LOWER(?)`, boardID, name).Scan(&existing)
	if existing > 0 {
		return nil, fmt.Errorf("CONFLICT: phase %q already exists on this board", name)
	}

	// Position: max + 1
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

	LogActivity(db, boardID, nil, "phase_created", fmt.Sprintf("Created phase: %s", name), "")

	return &models.Phase{ID: id, BoardID: boardID, Name: name, Color: color, Position: position, CreatedAt: now, UpdatedAt: now}, nil
}

func updatePhase(db *sqlx.DB, phaseID, name, color string) (*models.Phase, error) {
	if phaseID == "" {
		return nil, fmt.Errorf("VALIDATION: phase_id is required")
	}

	var phase models.Phase
	if err := db.Get(&phase, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE id = ?`, phaseID); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: phase not found")
	}

	if name != "" {
		if len(name) > 50 {
			return nil, fmt.Errorf("VALIDATION: name must be 50 characters or less")
		}
		// Check uniqueness excluding self
		var existing int
		db.QueryRow(`SELECT COUNT(*) FROM phases WHERE board_id = ? AND LOWER(name) = LOWER(?) AND id != ?`, phase.BoardID, name, phaseID).Scan(&existing)
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

	_, err := db.Exec(
		`UPDATE phases SET name = ?, color = ?, updated_at = ? WHERE id = ?`,
		phase.Name, phase.Color, phase.UpdatedAt, phase.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update phase: %w", err)
	}

	LogActivity(db, phase.BoardID, nil, "phase_updated", fmt.Sprintf("Updated phase: %s", phase.Name), "")

	return &phase, nil
}

func deletePhase(db *sqlx.DB, phaseID string) (map[string]interface{}, error) {
	if phaseID == "" {
		return nil, fmt.Errorf("VALIDATION: phase_id is required")
	}

	var phase models.Phase
	if err := db.Get(&phase, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE id = ?`, phaseID); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: phase not found")
	}

	if _, err := db.Exec(`DELETE FROM phases WHERE id = ?`, phaseID); err != nil {
		return nil, fmt.Errorf("delete phase: %w", err)
	}

	LogActivity(db, phase.BoardID, nil, "phase_deleted", fmt.Sprintf("Deleted phase: %s", phase.Name), "")

	return map[string]interface{}{"deleted": true, "phase_id": phaseID}, nil
}

func reorderPhases(db *sqlx.DB, boardID string, orderedIDs []string) ([]models.Phase, error) {
	if boardID == "" {
		board, err := ResolveBoard(db, "")
		if err != nil {
			return nil, err
		}
		boardID = board.ID
	}

	if len(orderedIDs) == 0 {
		return nil, fmt.Errorf("VALIDATION: ordered_ids is required for reorder")
	}

	// Validate all IDs belong to this board
	var phases []models.Phase
	if err := db.Select(&phases, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, boardID); err != nil {
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

	// Assign sequential positions
	now := time.Now().UTC().Format(time.RFC3339)
	for i, id := range orderedIDs {
		pos := float64(i + 1)
		if _, err := db.Exec(`UPDATE phases SET position = ?, updated_at = ? WHERE id = ?`, pos, now, id); err != nil {
			return nil, fmt.Errorf("reorder phase: %w", err)
		}
	}

	// Return updated list
	var result []models.Phase
	if err := db.Select(&result, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, boardID); err != nil {
		return nil, err
	}
	if result == nil {
		result = []models.Phase{}
	}

	LogActivity(db, boardID, nil, "phases_reordered", "Phases reordered", "")

	return result, nil
}
