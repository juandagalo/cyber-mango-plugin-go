package main

import (
	"fmt"
	"os"

	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	mcphandlers "github.com/juandagalo/cyber-mango-plugin-go/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	dbPath := db.ResolveDbPath()
	fmt.Fprintf(os.Stderr, "cyber-mango: connecting to db at %s\n", dbPath)

	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cyber-mango: failed to open db: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		fmt.Fprintf(os.Stderr, "cyber-mango: migration failed: %v\n", err)
		os.Exit(1)
	}

	if err := db.SeedDefaultBoard(database); err != nil {
		fmt.Fprintf(os.Stderr, "cyber-mango: seed failed: %v\n", err)
		os.Exit(1)
	}

	s := mcphandlers.NewServer(database)
	fmt.Fprintf(os.Stderr, "cyber-mango: starting MCP server (stdio)\n")

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "cyber-mango: server error: %v\n", err)
		os.Exit(1)
	}
}
