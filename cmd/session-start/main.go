package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/services"
)

func main() {
	dbPath := db.ResolveDbPath()

	database, err := db.Open(dbPath)
	if err != nil {
		// DB not available yet — silent exit (no board to show)
		os.Exit(0)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		os.Exit(0)
	}

	if err := db.SeedDefaultBoard(database); err != nil {
		os.Exit(0)
	}

	summary, err := services.GetBoardSummary(database, "")
	if err != nil {
		os.Exit(0)
	}

	// Build the system message (plain text — Claude Code does not render markdown in hook output)
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
			board, err := services.GetBoard(database, "")
			if err == nil {
				for _, col := range board.Columns {
					for _, card := range col.Cards {
						if card.Priority == "critical" {
							sb.WriteString(fmt.Sprintf("    - [%s] %s\n", col.Name, card.Title))
						}
					}
				}
			}
		}
		if summary.ByPriority["high"] > 0 {
			sb.WriteString(fmt.Sprintf("  HIGH: %d\n", summary.ByPriority["high"]))
			board, err := services.GetBoard(database, "")
			if err == nil {
				for _, col := range board.Columns {
					for _, card := range col.Cards {
						if card.Priority == "high" {
							sb.WriteString(fmt.Sprintf("    - [%s] %s\n", col.Name, card.Title))
						}
					}
				}
			}
		}
	}

	msg := sb.String()
	output := map[string]string{"systemMessage": msg}
	data, err := json.Marshal(output)
	if err != nil {
		os.Exit(0)
	}
	fmt.Println(string(data))
}
