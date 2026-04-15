package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// isResolved returns true if the value is non-empty and not an unexpanded template string.
// Claude Code passes .mcp.json env vars as literal strings when the underlying var is not set.
func isResolved(v string) bool {
	return v != "" && !strings.HasPrefix(v, "${")
}

// ResolveDbPath returns the database path using this priority:
// 1. CYBER_MANGO_DB_PATH env var
// 2. ~/.cyber-mango/kanban.db (shared by MCP server, hooks, and web UI)
//
// CLAUDE_PLUGIN_DATA is intentionally NOT used — hooks cannot reliably
// access it (no env field in hooks.json, inline substitution broken for
// SessionStart), which causes MCP server and hooks to diverge to different DBs.
func ResolveDbPath() string {
	if v := os.Getenv("CYBER_MANGO_DB_PATH"); isResolved(v) {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cyber-mango", "kanban.db")
}

// Open opens (or creates) the SQLite database at the given path and applies pragmas.
func Open(dbPath string) (*sqlx.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Apply pragmas on every connection
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", p, err)
		}
	}

	return db, nil
}
