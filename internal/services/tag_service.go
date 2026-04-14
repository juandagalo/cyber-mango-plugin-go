package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

// ManageTags dispatches to the appropriate tag operation.
func ManageTags(db *sqlx.DB, action, boardID, tagID, cardID, name, color string) (interface{}, error) {
	switch action {
	case "create":
		return createTag(db, boardID, name, color)
	case "assign":
		return assignTag(db, cardID, tagID)
	case "remove":
		return removeTag(db, cardID, tagID)
	case "list":
		return listTags(db, boardID)
	case "delete":
		return deleteTag(db, tagID)
	default:
		return nil, fmt.Errorf("VALIDATION: unknown action %q", action)
	}
}

func createTag(db *sqlx.DB, boardID, name, color string) (*models.Tag, error) {
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
	if color == "" {
		color = "#3b82f6"
	}
	if !strings.HasPrefix(color, "#") || len(color) != 7 {
		return nil, fmt.Errorf("VALIDATION: color must be a 7-character hex color (e.g. #3b82f6)")
	}

	id, _ := gonanoid.New(12)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(
		`INSERT INTO tags (id, board_id, name, color, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, boardID, name, color, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("CONFLICT: tag %q already exists on this board", name)
		}
		return nil, fmt.Errorf("insert tag: %w", err)
	}

	return &models.Tag{ID: id, BoardID: boardID, Name: name, Color: color, CreatedAt: now}, nil
}

// FindOrCreateTag returns an existing tag by name on the board, or creates it with the default color.
func FindOrCreateTag(db *sqlx.DB, boardID, name string) (*models.Tag, error) {
	if name == "" {
		return nil, fmt.Errorf("VALIDATION: tag name is required")
	}
	if boardID == "" {
		board, err := ResolveBoard(db, "")
		if err != nil {
			return nil, err
		}
		boardID = board.ID
	}

	var existing models.Tag
	err := db.Get(&existing, `SELECT id, board_id, name, color, created_at FROM tags WHERE board_id = ? AND LOWER(name) = LOWER(?)`, boardID, name)
	if err == nil {
		return &existing, nil
	}

	return createTag(db, boardID, name, "#3b82f6")
}

func assignTag(db *sqlx.DB, cardID, tagID string) (map[string]interface{}, error) {
	if cardID == "" || tagID == "" {
		return nil, fmt.Errorf("VALIDATION: card_id and tag_id are required")
	}
	_, err := db.Exec(`INSERT OR IGNORE INTO card_tags (card_id, tag_id) VALUES (?, ?)`, cardID, tagID)
	if err != nil {
		return nil, fmt.Errorf("assign tag: %w", err)
	}
	return map[string]interface{}{"assigned": true, "card_id": cardID, "tag_id": tagID}, nil
}

func removeTag(db *sqlx.DB, cardID, tagID string) (map[string]interface{}, error) {
	if cardID == "" || tagID == "" {
		return nil, fmt.Errorf("VALIDATION: card_id and tag_id are required")
	}
	_, err := db.Exec(`DELETE FROM card_tags WHERE card_id = ? AND tag_id = ?`, cardID, tagID)
	if err != nil {
		return nil, fmt.Errorf("remove tag: %w", err)
	}
	return map[string]interface{}{"removed": true, "card_id": cardID, "tag_id": tagID}, nil
}

func listTags(db *sqlx.DB, boardID string) ([]models.Tag, error) {
	if boardID == "" {
		board, err := ResolveBoard(db, "")
		if err != nil {
			return nil, err
		}
		boardID = board.ID
	}
	var tags []models.Tag
	if err := db.Select(&tags, `SELECT id, board_id, name, color, created_at FROM tags WHERE board_id = ? ORDER BY name`, boardID); err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []models.Tag{}
	}
	return tags, nil
}

func deleteTag(db *sqlx.DB, tagID string) (map[string]interface{}, error) {
	if tagID == "" {
		return nil, fmt.Errorf("VALIDATION: tag_id is required")
	}
	if _, err := db.Exec(`DELETE FROM tags WHERE id = ?`, tagID); err != nil {
		return nil, fmt.Errorf("delete tag: %w", err)
	}
	return map[string]interface{}{"deleted": true, "tag_id": tagID}, nil
}
