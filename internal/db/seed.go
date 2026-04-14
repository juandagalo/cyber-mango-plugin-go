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
		name     string
		color    string
		position float64
	}{
		{"Backlog", "#6b7280", 1000},
		{"To Do", "#3b82f6", 2000},
		{"In Progress", "#f59e0b", 3000},
		{"Review", "#8b5cf6", 4000},
		{"Done", "#10b981", 5000},
	}

	for _, col := range columns {
		colID, err := gonanoid.New(12)
		if err != nil {
			return fmt.Errorf("generate column id: %w", err)
		}
		_, err = db.Exec(
			`INSERT INTO columns (id, board_id, name, color, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			colID, boardID, col.name, col.color, col.position, now, now,
		)
		if err != nil {
			return fmt.Errorf("insert column %s: %w", col.name, err)
		}
	}

	return nil
}
