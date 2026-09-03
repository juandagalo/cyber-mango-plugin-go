package db

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// isResolved rejects empty values and unexpanded templates like ${VAR}, which Claude Code passes literally when the env var is unset.
func isResolved(v string) bool {
	return v != "" && !strings.HasPrefix(v, "${")
}

// ResolveDbPath returns CYBER_MANGO_DB_PATH if set, else ~/.cyber-mango/kanban.db.
// CLAUDE_PLUGIN_DATA is deliberately ignored: hooks cannot read it, so using it
// would send the MCP server and the hooks to different databases.
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

// SQLite pragmas are per-connection; the driver applies these on every pooled connection.
const dsnPragmas = "_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=foreign_keys(1)" +
	"&_pragma=synchronous(NORMAL)"

func Open(dbPath string) (*sqlx.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sqlx.Open("sqlite", dbPath+"?"+dsnPragmas)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Pragmas and :memory: are per-connection; one connection keeps them consistent.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open db: %w", err)
	}

	return db, nil
}
