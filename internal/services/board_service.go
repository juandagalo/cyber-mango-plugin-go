package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

func ListBoards(db *sqlx.DB) ([]models.Board, error) {
	boards := []models.Board{}
	err := db.Select(&boards, `SELECT id, name, description, created_at, updated_at FROM boards ORDER BY created_at`)
	return boards, err
}

// ResolveBoard falls back to the oldest board when boardID is empty.
func ResolveBoard(db Querier, boardID string) (*models.Board, error) {
	var board models.Board
	var query string
	var args []interface{}

	if boardID == "" {
		query = `SELECT id, name, description, created_at, updated_at FROM boards ORDER BY created_at LIMIT 1`
	} else {
		query = `SELECT id, name, description, created_at, updated_at FROM boards WHERE id = ?`
		args = []interface{}{boardID}
	}

	err := db.Get(&board, query, args...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("NOT_FOUND: board not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get board: %w", err)
	}
	return &board, nil
}

// resolveBoardID returns boardID unchanged when set, else the oldest board's ID.
func resolveBoardID(db Querier, boardID string) (string, error) {
	if boardID != "" {
		return boardID, nil
	}
	board, err := ResolveBoard(db, "")
	if err != nil {
		return "", err
	}
	return board.ID, nil
}

// ResolveColumn matches by ID, then by case-insensitive name, then falls back to the board's first column. It never errors on empty input.
func ResolveColumn(db Querier, boardID, columnID, columnName string) (*models.Column, error) {
	if columnID != "" {
		return getColumn(db, columnID)
	}

	cols, err := listColumns(db, boardID)
	if err != nil {
		return nil, err
	}

	if columnName != "" {
		for i := range cols {
			if strings.EqualFold(cols[i].Name, columnName) {
				return &cols[i], nil
			}
		}
		return nil, fmt.Errorf("NOT_FOUND: column %q not found", columnName)
	}

	if len(cols) == 0 {
		return nil, fmt.Errorf("NOT_FOUND: no columns on board")
	}
	return &cols[0], nil
}

// cardTagRow carries the owning card_id next to the tag so one query can load
// every tag assignment on a board.
type cardTagRow struct {
	CardID string `db:"card_id"`
	models.Tag
}

// GetBoard runs a fixed number of queries (board, phases, columns, cards, tags)
// regardless of board size and assembles the tree in memory. Columns and cards
// are ordered by position; tags on a card are ordered by name.
func GetBoard(db *sqlx.DB, boardID string) (*models.Board, error) {
	board, err := ResolveBoard(db, boardID)
	if err != nil {
		return nil, err
	}

	phases := []models.Phase{}
	if err := db.Select(&phases, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, board.ID); err != nil {
		return nil, fmt.Errorf("query phases: %w", err)
	}
	phaseMap := make(map[string]*models.Phase, len(phases))
	for i := range phases {
		phaseMap[phases[i].ID] = &phases[i]
	}
	board.Phases = phases

	columns, err := listColumns(db, board.ID)
	if err != nil {
		return nil, err
	}

	cards := []models.Card{}
	if err := db.Select(&cards, `SELECT c.id, c.column_id, c.title, c.description, c.priority, c.position, c.parent_card_id, c.due_date, c.phase_id, c.created_at, c.updated_at
		FROM cards c JOIN columns col ON col.id = c.column_id
		WHERE col.board_id = ? ORDER BY col.position, c.position`, board.ID); err != nil {
		return nil, fmt.Errorf("query cards: %w", err)
	}

	tagRows := []cardTagRow{}
	if err := db.Select(&tagRows, `SELECT ct.card_id, t.id, t.board_id, t.name, t.color, t.created_at
		FROM card_tags ct
		JOIN tags t ON t.id = ct.tag_id
		JOIN cards c ON c.id = ct.card_id
		JOIN columns col ON col.id = c.column_id
		WHERE col.board_id = ? ORDER BY ct.card_id, t.name`, board.ID); err != nil {
		return nil, fmt.Errorf("query card tags: %w", err)
	}
	tagsByCard := make(map[string][]models.Tag, len(tagRows))
	for _, row := range tagRows {
		tagsByCard[row.CardID] = append(tagsByCard[row.CardID], row.Tag)
	}

	cardsByColumn := make(map[string][]models.Card, len(columns))
	for i := range cards {
		card := cards[i]
		card.Tags = tagsByCard[card.ID]
		if card.Tags == nil {
			card.Tags = []models.Tag{}
		}
		if card.PhaseID != nil {
			card.Phase = phaseMap[*card.PhaseID]
		}
		cardsByColumn[card.ColumnID] = append(cardsByColumn[card.ColumnID], card)
	}

	for i := range columns {
		columns[i].Cards = cardsByColumn[columns[i].ID]
		if columns[i].Cards == nil {
			columns[i].Cards = []models.Card{}
		}
	}
	board.Columns = columns
	return board, nil
}

// cardCountRow is one bucket of the card aggregate: phase_id is NULL for unassigned cards.
type cardCountRow struct {
	ColumnID string  `db:"column_id"`
	Priority string  `db:"priority"`
	PhaseID  *string `db:"phase_id"`
	Count    int     `db:"cnt"`
}

// GetBoardSummary runs a fixed number of queries (board, columns, phases, one
// card aggregate) regardless of board size and folds the aggregate in memory.
func GetBoardSummary(db *sqlx.DB, boardID string) (*models.BoardSummary, error) {
	board, err := ResolveBoard(db, boardID)
	if err != nil {
		return nil, err
	}

	columns, err := listColumns(db, board.ID)
	if err != nil {
		return nil, err
	}

	phases := []models.Phase{}
	if err := db.Select(&phases, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, board.ID); err != nil {
		return nil, fmt.Errorf("query phases: %w", err)
	}
	phaseNames := make(map[string]string, len(phases))
	for _, p := range phases {
		phaseNames[p.ID] = p.Name
	}

	counts := []cardCountRow{}
	if err := db.Select(&counts, `SELECT c.column_id, c.priority, c.phase_id, COUNT(*) AS cnt
		FROM cards c JOIN columns col ON col.id = c.column_id
		WHERE col.board_id = ?
		GROUP BY c.column_id, c.priority, c.phase_id`, board.ID); err != nil {
		return nil, fmt.Errorf("query card counts: %w", err)
	}

	summary := &models.BoardSummary{
		BoardID:    board.ID,
		BoardName:  board.Name,
		Columns:    make([]models.ColumnSummary, 0, len(columns)),
		ByPriority: map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0},
		ByPhase:    map[string]int{},
	}

	countByColumn := make(map[string]int, len(columns))
	for _, row := range counts {
		summary.TotalCards += row.Count
		countByColumn[row.ColumnID] += row.Count
		summary.ByPriority[row.Priority] += row.Count
		if row.PhaseID == nil {
			summary.ByPhase["unassigned"] += row.Count
		} else if name, ok := phaseNames[*row.PhaseID]; ok {
			// ON DELETE SET NULL makes a dangling phase_id impossible; skip rather than invent a key.
			summary.ByPhase[name] += row.Count
		}
	}

	for _, col := range columns {
		summary.Columns = append(summary.Columns, models.ColumnSummary{
			ColumnID:    col.ID,
			ColumnName:  col.Name,
			Description: col.Description,
			CardCount:   countByColumn[col.ID],
			WipLimit:    col.WipLimit,
		})
	}

	return summary, nil
}
