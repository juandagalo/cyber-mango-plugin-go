package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/sqltx"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

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
	var tag *models.Tag
	err := sqltx.Run(db, func(tx *sqlx.Tx) error {
		var err error
		tag, err = createTagTx(tx, boardID, name, color)
		return err
	})
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// createTagTx writes the tag and its activity row, so q must be a *sqlx.Tx.
func createTagTx(q Querier, boardID, name, color string) (*models.Tag, error) {
	if boardID == "" {
		board, err := ResolveBoard(q, "")
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

	_, err := q.Exec(
		`INSERT INTO tags (id, board_id, name, color, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, boardID, name, color, now,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("CONFLICT: tag %q already exists on this board", name)
		}
		return nil, fmt.Errorf("insert tag: %w", err)
	}

	if err := LogActivity(q, boardID, nil, "tag_created", fmt.Sprintf("Created tag: %s", name), ""); err != nil {
		return nil, fmt.Errorf("log activity: %w", err)
	}

	return &models.Tag{ID: id, BoardID: boardID, Name: name, Color: color, CreatedAt: now}, nil
}

// FindOrCreateTag matches name case-insensitively and creates with the default
// color. Creating writes two rows (tag + activity), so q must be a *sqlx.Tx.
func FindOrCreateTag(q Querier, boardID, name string) (*models.Tag, error) {
	if name == "" {
		return nil, fmt.Errorf("VALIDATION: tag name is required")
	}
	if boardID == "" {
		board, err := ResolveBoard(q, "")
		if err != nil {
			return nil, err
		}
		boardID = board.ID
	}

	var existing models.Tag
	err := q.Get(&existing, `SELECT id, board_id, name, color, created_at FROM tags WHERE board_id = ? AND LOWER(name) = LOWER(?)`, boardID, name)
	if err == nil {
		return &existing, nil
	}

	return createTagTx(q, boardID, name, "#3b82f6")
}

func getTag(q Querier, tagID string) (*models.Tag, error) {
	var tag models.Tag
	err := q.Get(&tag, `SELECT id, board_id, name, color, created_at FROM tags WHERE id = ?`, tagID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("NOT_FOUND: tag not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get tag: %w", err)
	}
	return &tag, nil
}

func assignTag(db *sqlx.DB, cardID, tagID string) (map[string]interface{}, error) {
	if cardID == "" || tagID == "" {
		return nil, fmt.Errorf("VALIDATION: card_id and tag_id are required")
	}
	err := sqltx.Run(db, func(tx *sqlx.Tx) error {
		tag, err := getTag(tx, tagID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO card_tags (card_id, tag_id) VALUES (?, ?)`, cardID, tagID); err != nil {
			return fmt.Errorf("assign tag: %w", err)
		}
		if err := LogActivity(tx, tag.BoardID, &cardID, "tag_assigned", fmt.Sprintf("Assigned tag: %s", tag.Name), ""); err != nil {
			return fmt.Errorf("log activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"assigned": true, "card_id": cardID, "tag_id": tagID}, nil
}

func removeTag(db *sqlx.DB, cardID, tagID string) (map[string]interface{}, error) {
	if cardID == "" || tagID == "" {
		return nil, fmt.Errorf("VALIDATION: card_id and tag_id are required")
	}
	err := sqltx.Run(db, func(tx *sqlx.Tx) error {
		tag, err := getTag(tx, tagID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM card_tags WHERE card_id = ? AND tag_id = ?`, cardID, tagID); err != nil {
			return fmt.Errorf("remove tag: %w", err)
		}
		if err := LogActivity(tx, tag.BoardID, &cardID, "tag_removed", fmt.Sprintf("Removed tag: %s", tag.Name), ""); err != nil {
			return fmt.Errorf("log activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	err := sqltx.Run(db, func(tx *sqlx.Tx) error {
		tag, err := getTag(tx, tagID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM tags WHERE id = ?`, tagID); err != nil {
			return fmt.Errorf("delete tag: %w", err)
		}
		if err := LogActivity(tx, tag.BoardID, nil, "tag_deleted", fmt.Sprintf("Deleted tag: %s", tag.Name), ""); err != nil {
			return fmt.Errorf("log activity: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"deleted": true, "tag_id": tagID}, nil
}
