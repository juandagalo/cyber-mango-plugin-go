package services

import (
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

func LogActivity(db Querier, boardID string, cardID *string, action, details, agent string) error {
	id, err := gonanoid.New(12)
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	var detailsVal, agentVal *string
	if details != "" {
		detailsVal = &details
	}
	if agent != "" {
		agentVal = &agent
	}

	_, err = db.Exec(
		`INSERT INTO activity_log (id, board_id, card_id, action, details, agent, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, boardID, cardID, action, detailsVal, agentVal, now,
	)
	return err
}
