package db

import (
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/jmoiron/sqlx"
)

// SeedDefaultBoard creates the default "Cyber Mango" board if no boards exist.
func SeedDefaultBoard(db *sqlx.DB) error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM boards`).Scan(&count); err != nil {
		return fmt.Errorf("count boards: %w", err)
	}
	if count > 0 {
		return nil
	}

	boardID, err := gonanoid.New(12)
	if err != nil {
		return fmt.Errorf("generate board id: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO boards (id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		boardID, "Cyber Mango", "Default kanban board", now, now,
	)
	if err != nil {
		return fmt.Errorf("insert board: %w", err)
	}

	columns := []struct {
		name        string
		color       string
		position    float64
		description string
	}{
		{"Backlog", "#6b7280", 1000, "Ideas, future work, and parked items not yet committed to. Default column for new cards when no specific column is indicated."},
		{"To Do", "#3b82f6", 2000, "Committed work ready to start in the near term. Cards here have been prioritized and are waiting to be picked up."},
		{"In Progress", "#f59e0b", 3000, "Work actively being done right now. Keep this column small to maintain focus."},
		{"Review", "#8b5cf6", 4000, "Work complete from the implementer side, waiting for code review, QA, or client approval."},
		{"Done", "#10b981", 5000, "Completed, verified, and deployed. The work is fully finished and live."},
	}

	for _, col := range columns {
		colID, err := gonanoid.New(12)
		if err != nil {
			return fmt.Errorf("generate column id: %w", err)
		}
		_, err = db.Exec(
			`INSERT INTO columns (id, board_id, name, color, position, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			colID, boardID, col.name, col.color, col.position, col.description, now, now,
		)
		if err != nil {
			return fmt.Errorf("insert column %s: %w", col.name, err)
		}
	}

	phases := []struct {
		name     string
		color    string
		position float64
	}{
		{"Development", "#00FFFF", 1.0},
		{"Code Review", "#BF00FF", 2.0},
		{"QA", "#FCEE0A", 3.0},
		{"Client Review", "#FF00FF", 4.0},
		{"Ready to Deploy", "#39FF14", 5.0},
	}

	for _, ph := range phases {
		phID, err := gonanoid.New(12)
		if err != nil {
			return fmt.Errorf("generate phase id: %w", err)
		}
		_, err = db.Exec(
			`INSERT INTO phases (id, board_id, name, color, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			phID, boardID, ph.name, ph.color, ph.position, now, now,
		)
		if err != nil {
			return fmt.Errorf("insert phase %s: %w", ph.name, err)
		}
	}

	return nil
}
