package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/models"
)

const metaKey = "last_stop_report"

func main() {
	dbPath := db.ResolveDbPath()

	database, err := db.Open(dbPath)
	if err != nil {
		os.Exit(0)
	}
	defer database.Close()

	var since string
	database.QueryRow(`SELECT value FROM _meta WHERE key = ?`, metaKey).Scan(&since)

	activities := queryNewActivity(database, since)
	if len(activities) == 0 {
		os.Exit(0)
	}

	newest := activities[0].CreatedAt // ordered DESC, first is newest
	database.Exec(`INSERT OR REPLACE INTO _meta (key, value) VALUES (?, ?)`, metaKey, newest)

	counts := map[string]int{}
	for _, a := range activities {
		counts[a.Action]++
	}

	var sb strings.Builder
	actionLabels := []struct{ key, label string }{
		{"card_created", "Cards created"},
		{"card_updated", "Cards updated"},
		{"card_moved", "Cards moved"},
		{"card_deleted", "Cards deleted"},
		{"column_created", "Columns created"},
		{"phase_created", "Phases created"},
		{"phase_updated", "Phases updated"},
		{"phase_deleted", "Phases deleted"},
		{"phases_reordered", "Phases reordered"},
	}
	for _, al := range actionLabels {
		if n := counts[al.key]; n > 0 {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", al.label, n))
		}
	}

	output := map[string]string{"systemMessage": sb.String()}
	data, err := json.Marshal(output)
	if err != nil {
		os.Exit(0)
	}
	fmt.Println(string(data))
}

func queryNewActivity(db *sqlx.DB, since string) []models.ActivityLog {
	var logs []models.ActivityLog
	if since == "" {
		db.Select(&logs, `SELECT id, board_id, card_id, action, details, agent, created_at FROM activity_log ORDER BY created_at DESC`)
	} else {
		db.Select(&logs, `SELECT id, board_id, card_id, action, details, agent, created_at FROM activity_log WHERE created_at > ? ORDER BY created_at DESC`, since)
	}
	return logs
}
