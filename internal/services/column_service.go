package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

// CreateColumn creates a new column on a board.
func CreateColumn(db *sqlx.DB, boardID, name, color, description string, wipLimit *int) (*models.Column, error) {
	board, err := ResolveBoard(db, boardID)
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("VALIDATION: name is required")
	}
	if color == "" {
		color = "#6b7280"
	}

	// Convert empty/whitespace-only description to nil (stored as NULL).
	var descPtr *string
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		descPtr = &trimmed
	}

	// Position: max + 1000
	var maxPos float64
	db.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM columns WHERE board_id = ?`, board.ID).Scan(&maxPos)
	position := maxPos + 1000

	id, _ := gonanoid.New(12)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.Exec(
		`INSERT INTO columns (id, board_id, name, color, description, wip_limit, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, board.ID, name, color, descPtr, wipLimit, position, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert column: %w", err)
	}

	col := &models.Column{
		ID: id, BoardID: board.ID, Name: name, Color: color,
		Description: descPtr, WipLimit: wipLimit, Position: position,
		CreatedAt: now, UpdatedAt: now,
		Cards: []models.Card{},
	}

	LogActivity(db, board.ID, nil, "column_created", fmt.Sprintf("Created column: %s", name), "")
	return col, nil
}
