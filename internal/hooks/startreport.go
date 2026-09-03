package hooks

import (
	"fmt"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
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
// render markdown in hook output. It loads the board once and derives every
// count from that tree.
func StartReport(db *sqlx.DB) (string, error) {
	board, err := services.GetBoard(db, "")
	if err != nil {
		return "", err
	}

	total := 0
	byPriority := map[string]int{}
	byPhaseID := map[string]int{}
	unassigned := 0
	for _, col := range board.Columns {
		total += len(col.Cards)
		for _, card := range col.Cards {
			byPriority[card.Priority]++
			switch {
			case card.PhaseID == nil:
				unassigned++
			case card.Phase != nil:
				byPhaseID[card.Phase.ID]++
			}
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cyber Mango Board: %s\n\n", board.Name))
	sb.WriteString(fmt.Sprintf("Total cards: %d\n\n", total))

	sb.WriteString("Columns\n")
	for _, col := range board.Columns {
		count := len(col.Cards)
		wipStr := ""
		if col.WipLimit != nil {
			wipStr = fmt.Sprintf(" (WIP: %d/%d)", count, *col.WipLimit)
		}
		if col.Description != nil {
			sb.WriteString(fmt.Sprintf("  %s (%d cards)%s: %s\n", col.Name, count, wipStr, *col.Description))
		} else {
			sb.WriteString(fmt.Sprintf("  %s: %d cards%s\n", col.Name, count, wipStr))
		}
	}

	if len(byPhaseID) > 0 || unassigned > 0 {
		sb.WriteString("\nBy Phase\n")
		for _, phase := range board.Phases {
			if n := byPhaseID[phase.ID]; n > 0 {
				sb.WriteString(fmt.Sprintf("  %s: %d\n", phase.Name, n))
			}
		}
		if unassigned > 0 {
			sb.WriteString(fmt.Sprintf("  unassigned: %d\n", unassigned))
		}
	}

	if byPriority["critical"] > 0 || byPriority["high"] > 0 {
		sb.WriteString("\nPriority Alerts\n")
		for _, priority := range []string{"critical", "high"} {
			if n := byPriority[priority]; n > 0 {
				sb.WriteString(fmt.Sprintf("  %s: %d\n", strings.ToUpper(priority), n))
				writeCardsWithPriority(board, &sb, priority)
			}
		}
	}

	return sb.String(), nil
}

func writeCardsWithPriority(board *models.Board, sb *strings.Builder, priority string) {
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			if card.Priority == priority {
				sb.WriteString(fmt.Sprintf("    - [%s] %s\n", col.Name, card.Title))
			}
		}
	}
}
