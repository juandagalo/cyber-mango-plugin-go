package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/services"
)

func parseDbPath() string {
	for i, arg := range os.Args {
		if arg == "--db-path" && i+1 < len(os.Args) {
			v := os.Args[i+1]
			if v != "" && !strings.HasPrefix(v, "${") {
				return v
			}
		}
	}
	return ""
}

func main() {
	dbPath := parseDbPath()
	if dbPath == "" {
		dbPath = db.ResolveDbPath()
	}

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
		sb.WriteString(fmt.Sprintf("  %s: %d cards%s\n", col.ColumnName, col.CardCount, wipStr))
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
