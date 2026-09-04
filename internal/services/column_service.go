package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

const columnSelect = `SELECT id, board_id, name, color, description, wip_limit, position, created_at, updated_at FROM columns`

func getColumn(q Querier, id string) (*models.Column, error) {
	var col models.Column
	err := q.Get(&col, columnSelect+` WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("NOT_FOUND: column not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get column: %w", err)
	}
	return &col, nil
}

// listColumns returns the board's columns ordered by position; never nil.
func listColumns(q Querier, boardID string) ([]models.Column, error) {
	cols := []models.Column{}
	if err := q.Select(&cols, columnSelect+` WHERE board_id = ? ORDER BY position`, boardID); err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}
	return cols, nil
}

func CreateColumn(db *sqlx.DB, boardID, name, color, description string, wipLimit *int) (*models.Column, error) {
	board, err := ResolveBoard(db, boardID)
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, fmt.Errorf("VALIDATION: name is required")
	}
	color, err = normalizeColor(color, "#6b7280")
	if err != nil {
		return nil, err
	}

	var descPtr *string
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		descPtr = &trimmed
	}

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

	if err := LogActivity(db, board.ID, nil, "column_created", fmt.Sprintf("Created column: %s", name), ""); err != nil {
		return nil, fmt.Errorf("log activity: %w", err)
	}
	return col, nil
}
