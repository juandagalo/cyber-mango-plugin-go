package db

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

const currentSchemaVersion = "1"

// RunMigrations ensures the schema is up to date.
func RunMigrations(db *sqlx.DB) error {
	// Ensure meta table exists
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _meta (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return fmt.Errorf("create _meta: %w", err)
	}

	var version string
	err := db.QueryRow(`SELECT value FROM _meta WHERE key = 'schema_version'`).Scan(&version)
	if err != nil {
		// Table doesn't have version yet — run full schema
		if err := createSchema(db); err != nil {
			return err
		}
		if _, err = db.Exec(`INSERT INTO _meta (key, value) VALUES ('schema_version', ?)`, currentSchemaVersion); err != nil {
			return err
		}
	}

	// Future migrations: check version and ALTER TABLE as needed

	// Ensure Drizzle migration journal exists so the web UI won't re-run CREATE TABLE.
	// The web UI uses Drizzle ORM which tracks applied migrations in __drizzle_migrations.
	// Without this, whoever touches the DB first (Go plugin vs web UI) breaks the other.
	if err := ensureDrizzleJournal(db); err != nil {
		return fmt.Errorf("drizzle journal: %w", err)
	}

	return nil
}

// ensureDrizzleJournal creates the __drizzle_migrations table and marks the
// initial migration as applied, so Drizzle ORM (used by the web UI) recognizes
// that the schema already exists and skips CREATE TABLE statements.
func ensureDrizzleJournal(db *sqlx.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS __drizzle_migrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hash TEXT NOT NULL,
		created_at BIGINT
	)`)
	if err != nil {
		return err
	}

	// Check if the initial migration is already recorded
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM __drizzle_migrations WHERE hash = '0000_wandering_sister_grimm'`).Scan(&count)
	if count == 0 {
		_, err = db.Exec(`INSERT INTO __drizzle_migrations (hash, created_at) VALUES ('0000_wandering_sister_grimm', 1776186662950)`)
	}
	return err
}

func createSchema(db *sqlx.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS boards (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS columns (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  color TEXT DEFAULT '#6b7280',
  wip_limit INTEGER,
  position REAL NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_columns_board_position ON columns(board_id, position);

CREATE TABLE IF NOT EXISTS cards (
  id TEXT PRIMARY KEY,
  column_id TEXT NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT DEFAULT '',
  priority TEXT DEFAULT 'medium' CHECK(priority IN ('low','medium','high','critical')),
  position REAL NOT NULL,
  parent_card_id TEXT,
  due_date TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_cards_column_position ON cards(column_id, position);

CREATE TABLE IF NOT EXISTS tags (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  color TEXT NOT NULL DEFAULT '#3b82f6',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_tags_board ON tags(board_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_board_name ON tags(board_id, name);

CREATE TABLE IF NOT EXISTS card_tags (
  card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
  tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (card_id, tag_id)
);
CREATE INDEX IF NOT EXISTS idx_card_tags_card ON card_tags(card_id);
CREATE INDEX IF NOT EXISTS idx_card_tags_tag ON card_tags(tag_id);

CREATE TABLE IF NOT EXISTS activity_log (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
  card_id TEXT,
  action TEXT NOT NULL,
  details TEXT,
  agent TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`
	_, err := db.Exec(schema)
	return err
}
