package hooks

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/services"
)

// StdinSessionID reads session_id from the hook payload on stdin, skipping the
// read when stdin is a terminal so a manual run does not block.
func StdinSessionID() string {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return ""
	}
	return ReadSessionID(os.Stdin)
}

// StartReport renders the board summary as plain text: Claude Code does not
// render markdown in hook output.
func StartReport(db *sqlx.DB) (string, error) {
	summary, err := services.GetBoardSummary(db, "")
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cyber Mango Board: %s\n\n", summary.BoardName))
	sb.WriteString(fmt.Sprintf("Total cards: %d\n\n", summary.TotalCards))

	sb.WriteString("Columns\n")
	for _, col := range summary.Columns {
		wipStr := ""
		if col.WipLimit != nil {
			wipStr = fmt.Sprintf(" (WIP: %d/%d)", col.CardCount, *col.WipLimit)
		}
		if col.Description != nil {
			sb.WriteString(fmt.Sprintf("  %s (%d cards)%s: %s\n", col.ColumnName, col.CardCount, wipStr, *col.Description))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: %d cards%s\n", col.ColumnName, col.CardCount, wipStr))
		}
	}

	if len(summary.ByPhase) > 0 {
		sb.WriteString("\nBy Phase\n")
		for phase, count := range summary.ByPhase {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", phase, count))
		}
	}

	if summary.ByPriority["critical"] > 0 || summary.ByPriority["high"] > 0 {
		sb.WriteString("\nPriority Alerts\n")
		if summary.ByPriority["critical"] > 0 {
			sb.WriteString(fmt.Sprintf("  CRITICAL: %d\n", summary.ByPriority["critical"]))
			writeCardsWithPriority(db, &sb, "critical")
		}
		if summary.ByPriority["high"] > 0 {
			sb.WriteString(fmt.Sprintf("  HIGH: %d\n", summary.ByPriority["high"]))
			writeCardsWithPriority(db, &sb, "high")
		}
	}

	return sb.String(), nil
}

func writeCardsWithPriority(db *sqlx.DB, sb *strings.Builder, priority string) {
	board, err := services.GetBoard(db, "")
	if err != nil {
		return
	}
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			if card.Priority == priority {
				sb.WriteString(fmt.Sprintf("    - [%s] %s\n", col.Name, card.Title))
			}
		}
	}
}
