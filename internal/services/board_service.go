package services

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

// ListBoards returns all boards.
func ListBoards(db *sqlx.DB) ([]models.Board, error) {
	var boards []models.Board
	err := db.Select(&boards, `SELECT id, name, description, created_at, updated_at FROM boards ORDER BY created_at`)
	return boards, err
}

// ResolveBoard returns the first board if boardID is empty, otherwise the specified board.
func ResolveBoard(db *sqlx.DB, boardID string) (*models.Board, error) {
	var board models.Board
	var query string
	var args []interface{}

	if boardID == "" {
		query = `SELECT id, name, description, created_at, updated_at FROM boards ORDER BY created_at LIMIT 1`
	} else {
		query = `SELECT id, name, description, created_at, updated_at FROM boards WHERE id = ?`
		args = []interface{}{boardID}
	}

	if err := db.Get(&board, query, args...); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: board not found")
	}
	return &board, nil
}

// ResolveColumn finds a column by ID or name (case-insensitive) within a board.
func ResolveColumn(db *sqlx.DB, boardID, columnID, columnName string) (*models.Column, error) {
	var col models.Column

	if columnID != "" {
		if err := db.Get(&col, `SELECT id, board_id, name, color, description, wip_limit, position, created_at, updated_at FROM columns WHERE id = ?`, columnID); err != nil {
			return nil, fmt.Errorf("NOT_FOUND: column not found")
		}
		return &col, nil
	}

	if columnName != "" {
		cols := []models.Column{}
		if err := db.Select(&cols, `SELECT id, board_id, name, color, description, wip_limit, position, created_at, updated_at FROM columns WHERE board_id = ? ORDER BY position`, boardID); err != nil {
			return nil, fmt.Errorf("NOT_FOUND: columns not found")
		}
		lower := strings.ToLower(columnName)
		for _, c := range cols {
			if strings.ToLower(c.Name) == lower {
				col = c
				return &col, nil
			}
		}
		return nil, fmt.Errorf("NOT_FOUND: column %q not found", columnName)
	}

	// Default: first column on the board
	if err := db.Get(&col, `SELECT id, board_id, name, color, description, wip_limit, position, created_at, updated_at FROM columns WHERE board_id = ? ORDER BY position LIMIT 1`, boardID); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: no columns on board")
	}
	return &col, nil
}

// GetBoard returns a board with nested columns and cards (including tags).
func GetBoard(db *sqlx.DB, boardID string) (*models.Board, error) {
	board, err := ResolveBoard(db, boardID)
	if err != nil {
		return nil, err
	}

	// Fetch phases for this board
	var phases []models.Phase
	if err := db.Select(&phases, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, board.ID); err != nil {
		return nil, fmt.Errorf("query phases: %w", err)
	}
	if phases == nil {
		phases = []models.Phase{}
	}
	// Build phaseMap for O(1) lookup
	phaseMap := make(map[string]*models.Phase, len(phases))
	for i := range phases {
		phaseMap[phases[i].ID] = &phases[i]
	}
	board.Phases = phases

	var columns []models.Column
	if err := db.Select(&columns, `SELECT id, board_id, name, color, description, wip_limit, position, created_at, updated_at FROM columns WHERE board_id = ? ORDER BY position`, board.ID); err != nil {
		return nil, fmt.Errorf("query columns: %w", err)
	}

	for i := range columns {
		var cards []models.Card
		if err := db.Select(&cards, `SELECT id, column_id, title, description, priority, position, parent_card_id, due_date, phase_id, created_at, updated_at FROM cards WHERE column_id = ? ORDER BY position`, columns[i].ID); err != nil {
			return nil, fmt.Errorf("query cards: %w", err)
		}

		for j := range cards {
			var tags []models.Tag
			if err := db.Select(&tags, `SELECT t.id, t.board_id, t.name, t.color, t.created_at FROM tags t JOIN card_tags ct ON ct.tag_id = t.id WHERE ct.card_id = ?`, cards[j].ID); err != nil {
				return nil, fmt.Errorf("query tags for card: %w", err)
			}
			if tags == nil {
				tags = []models.Tag{}
			}
			cards[j].Tags = tags

			// Populate phase from map
			if cards[j].PhaseID != nil {
				if p, ok := phaseMap[*cards[j].PhaseID]; ok {
					cards[j].Phase = p
				}
			}
		}
		if cards == nil {
			cards = []models.Card{}
		}
		columns[i].Cards = cards
	}
	if columns == nil {
		columns = []models.Column{}
	}
	board.Columns = columns
	return board, nil
}

// GetBoardSummary returns card counts per column and per priority.
func GetBoardSummary(db *sqlx.DB, boardID string) (*models.BoardSummary, error) {
	board, err := ResolveBoard(db, boardID)
	if err != nil {
		return nil, err
	}

	var columns []models.Column
	if err := db.Select(&columns, `SELECT id, board_id, name, color, description, wip_limit, position, created_at, updated_at FROM columns WHERE board_id = ? ORDER BY position`, board.ID); err != nil {
		return nil, err
	}

	// Fetch phases for name lookup
	var phases []models.Phase
	db.Select(&phases, `SELECT id, board_id, name, color, position, created_at, updated_at FROM phases WHERE board_id = ? ORDER BY position`, board.ID)
	phaseNameMap := make(map[string]string, len(phases))
	for _, p := range phases {
		phaseNameMap[p.ID] = p.Name
	}

	summary := &models.BoardSummary{
		BoardID:    board.ID,
		BoardName:  board.Name,
		ByPriority: map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0},
		ByPhase:    map[string]int{},
	}

	for _, col := range columns {
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM cards WHERE column_id = ?`, col.ID).Scan(&count)
		summary.TotalCards += count
		colSummary := models.ColumnSummary{
			ColumnID:    col.ID,
			ColumnName:  col.Name,
			Description: col.Description,
			CardCount:   count,
			WipLimit:    col.WipLimit,
		}
		summary.Columns = append(summary.Columns, colSummary)

		// Count by priority
		rows, _ := db.Queryx(`SELECT priority, COUNT(*) as cnt FROM cards WHERE column_id = ? GROUP BY priority`, col.ID)
		if rows != nil {
			for rows.Next() {
				var priority string
				var cnt int
				rows.Scan(&priority, &cnt)
				summary.ByPriority[priority] += cnt
			}
			rows.Close()
		}

		// Count by phase
		phaseRows, _ := db.Queryx(`SELECT phase_id, COUNT(*) as cnt FROM cards WHERE column_id = ? GROUP BY phase_id`, col.ID)
		if phaseRows != nil {
			for phaseRows.Next() {
				var phaseID *string
				var cnt int
				phaseRows.Scan(&phaseID, &cnt)
				if phaseID == nil {
					summary.ByPhase["unassigned"] += cnt
				} else if name, ok := phaseNameMap[*phaseID]; ok {
					summary.ByPhase[name] += cnt
				}
			}
			phaseRows.Close()
		}
	}
	if summary.Columns == nil {
		summary.Columns = []models.ColumnSummary{}
	}

	return summary, nil
}
