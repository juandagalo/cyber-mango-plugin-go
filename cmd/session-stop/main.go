package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/hooks"
)

func main() {
	sessionID := hooks.StdinSessionID()

	database, err := db.Open(db.ResolveDbPath())
	if err != nil {
		// Hooks must exit 0 with no output on any failure.
		os.Exit(0)
	}
	defer database.Close()

	msg, err := hooks.StopReport(database, sessionID, time.Now())
	if err != nil || msg == "" {
		os.Exit(0)
	}

	data, err := json.Marshal(map[string]string{"systemMessage": msg})
	if err != nil {
		os.Exit(0)
	}
	fmt.Println(string(data))
}
