package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

// CreateCard auto-creates and assigns each comma-separated name in tags.
func CreateCard(db *sqlx.DB, boardID, columnID, columnName, title, description, priority, tags, phaseID, phaseName string) (*models.Card, error) {
	board, err := ResolveBoard(db, boardID)
	if err != nil {
		return nil, err
	}

	col, err := ResolveColumn(db, board.ID, columnID, columnName)
	if err != nil {
		return nil, err
	}

	if title == "" {
		return nil, fmt.Errorf("VALIDATION: title is required")
	}

	validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
	if priority == "" {
		priority = "medium"
	} else if !validPriorities[priority] {
		return nil, fmt.Errorf("VALIDATION: invalid priority %q", priority)
	}

	var resolvedPhaseID *string
	if phaseID != "" || phaseName != "" {
		phase, err := ResolvePhase(db, board.ID, phaseID, phaseName)
		if err != nil {
			return nil, err
		}
		if phase != nil {
			resolvedPhaseID = &phase.ID
		}
	}

	var maxPos float64
	db.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM cards WHERE column_id = ?`, col.ID).Scan(&maxPos)
	position := maxPos + 1

	id, _ := gonanoid.New(12)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = db.Exec(
		`INSERT INTO cards (id, column_id, title, description, priority, position, phase_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, col.ID, title, description, priority, position, resolvedPhaseID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert card: %w", err)
	}

	card := &models.Card{
		ID: id, ColumnID: col.ID, Title: title, Description: description,
		Priority: priority, Position: position, PhaseID: resolvedPhaseID,
		CreatedAt: now, UpdatedAt: now,
		Tags: []models.Tag{},
	}

	LogActivity(db, board.ID, &id, "card_created", fmt.Sprintf("Created card: %s", title), "")

	if tags != "" {
		for _, raw := range strings.Split(tags, ",") {
			tagName := strings.TrimSpace(raw)
			if tagName == "" {
				continue
			}
			tag, err := FindOrCreateTag(db, board.ID, tagName)
			if err != nil {
				continue
			}
			db.Exec(`INSERT OR IGNORE INTO card_tags (card_id, tag_id) VALUES (?, ?)`, id, tag.ID)
			card.Tags = append(card.Tags, *tag)
		}
	}

	return card, nil
}

// UpdateCard treats empty strings as "unchanged" and moves the card when columnID or columnName is set.
func UpdateCard(db *sqlx.DB, cardID, title, description, priority, phaseID, phaseName string, unsetPhase bool, boardID, columnID, columnName string) (*models.Card, error) {
	var card models.Card
	if err := db.Get(&card, `SELECT id, column_id, title, description, priority, position, parent_card_id, due_date, phase_id, created_at, updated_at FROM cards WHERE id = ?`, cardID); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: card not found")
	}

	var resolvedBoardID string
	db.QueryRow(`SELECT c.board_id FROM columns c JOIN cards ca ON ca.column_id = c.id WHERE ca.id = ?`, cardID).Scan(&resolvedBoardID)
	if boardID == "" {
		boardID = resolvedBoardID
	}

	validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true, "": true}
	if !validPriorities[priority] {
		return nil, fmt.Errorf("VALIDATION: invalid priority %q", priority)
	}

	if title != "" {
		card.Title = title
	}
	if description != "" {
		card.Description = description
	}
	if priority != "" {
		card.Priority = priority
	}

	if unsetPhase {
		card.PhaseID = nil
	} else if phaseID != "" || phaseName != "" {
		phase, err := ResolvePhase(db, boardID, phaseID, phaseName)
		if err != nil {
			return nil, err
		}
		if phase != nil {
			card.PhaseID = &phase.ID
		}
	}

	moved := false
	if columnID != "" || columnName != "" {
		col, err := ResolveColumn(db, boardID, columnID, columnName)
		if err != nil {
			return nil, err
		}
		if col.ID != card.ColumnID {
			var maxPos float64
			db.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM cards WHERE column_id = ?`, col.ID).Scan(&maxPos)
			card.Position = maxPos + 1
			card.ColumnID = col.ID
			moved = true
		}
	}

	card.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(
		`UPDATE cards SET title = ?, description = ?, priority = ?, phase_id = ?, column_id = ?, position = ?, updated_at = ? WHERE id = ?`,
		card.Title, card.Description, card.Priority, card.PhaseID, card.ColumnID, card.Position, card.UpdatedAt, card.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update card: %w", err)
	}

	LogActivity(db, boardID, &cardID, "card_updated", fmt.Sprintf("Updated card: %s", card.Title), "")
	if moved {
		var colName string
		db.QueryRow(`SELECT name FROM columns WHERE id = ?`, card.ColumnID).Scan(&colName)
		LogActivity(db, boardID, &cardID, "card_moved", fmt.Sprintf("Moved card to column: %s", colName), "")
	}

	card.Tags = []models.Tag{}
	return &card, nil
}

// MoveCard with no column repositions within the current column.
func MoveCard(db *sqlx.DB, cardID, boardID, columnID, columnName string, position *float64) (*models.Card, error) {
	var card models.Card
	if err := db.Get(&card, `SELECT id, column_id, title, description, priority, position, parent_card_id, due_date, phase_id, created_at, updated_at FROM cards WHERE id = ?`, cardID); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: card not found")
	}

	var currentCol models.Column
	if err := db.Get(&currentCol, `SELECT id, board_id, name, color, description, wip_limit, position, created_at, updated_at FROM columns WHERE id = ?`, card.ColumnID); err != nil {
		return nil, fmt.Errorf("NOT_FOUND: current column not found")
	}

	col := &currentCol
	if columnID != "" || columnName != "" {
		if boardID == "" {
			boardID = currentCol.BoardID
		}
		target, err := ResolveColumn(db, boardID, columnID, columnName)
		if err != nil {
			return nil, err
		}
		col = target
	}

	newPosition := card.Position
	if position != nil {
		newPosition = *position
	} else if col.ID != card.ColumnID {
		var maxPos float64
		db.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM cards WHERE column_id = ?`, col.ID).Scan(&maxPos)
		newPosition = maxPos + 1
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(
		`UPDATE cards SET column_id = ?, position = ?, updated_at = ? WHERE id = ?`,
		col.ID, newPosition, now, cardID,
	)
	if err != nil {
		return nil, fmt.Errorf("move card: %w", err)
	}

	card.ColumnID = col.ID
	card.Position = newPosition
	card.UpdatedAt = now

	LogActivity(db, col.BoardID, &cardID, "card_moved", fmt.Sprintf("Moved card to column: %s", col.Name), "")

	card.Tags = []models.Tag{}
	return &card, nil
}

// DeleteCard relies on ON DELETE CASCADE for card_tags.
func DeleteCard(db *sqlx.DB, cardID string) error {
	var boardID string
	err := db.QueryRow(`SELECT c.board_id FROM columns c JOIN cards ca ON ca.column_id = c.id WHERE ca.id = ?`, cardID).Scan(&boardID)
	if err != nil {
		return fmt.Errorf("NOT_FOUND: card not found")
	}

	if _, err := db.Exec(`DELETE FROM cards WHERE id = ?`, cardID); err != nil {
		return fmt.Errorf("delete card: %w", err)
	}

	LogActivity(db, boardID, &cardID, "card_deleted", "Card deleted", "")
	return nil
}
