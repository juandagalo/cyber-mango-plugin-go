package db

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

const currentSchemaVersion = "3"

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

	// Migrate v1 -> v2: add phases table + cards.phase_id
	if version == "1" {
		if err := migrateV1ToV2(db); err != nil {
			return err
		}
		version = "2"
	}

	// Migrate v2 -> v3: add description column to columns table
	if version == "2" {
		if err := migrateV2ToV3(db); err != nil {
			return err
		}
		version = "3"
	}

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

	// Mark initial migration as applied
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM __drizzle_migrations WHERE hash = '0000_wandering_sister_grimm'`).Scan(&count)
	if count == 0 {
		if _, err = db.Exec(`INSERT INTO __drizzle_migrations (hash, created_at) VALUES ('0000_wandering_sister_grimm', 1776186662950)`); err != nil {
			return err
		}
	}

	// Mark phases migration as applied (web UI's 0001_right_polaris)
	db.QueryRow(`SELECT COUNT(*) FROM __drizzle_migrations WHERE hash = '0001_right_polaris'`).Scan(&count)
	if count == 0 {
		if _, err = db.Exec(`INSERT INTO __drizzle_migrations (hash, created_at) VALUES ('0001_right_polaris', 1776299103511)`); err != nil {
			return err
		}
	}

	// Mark cards restructure migration as applied (web UI's 0002_old_vengeance)
	db.QueryRow(`SELECT COUNT(*) FROM __drizzle_migrations WHERE hash = '0002_old_vengeance'`).Scan(&count)
	if count == 0 {
		if _, err = db.Exec(`INSERT INTO __drizzle_migrations (hash, created_at) VALUES ('0002_old_vengeance', 1776799182756)`); err != nil {
			return err
		}
	}

	return nil
}

func migrateV1ToV2(db *sqlx.DB) error {
	// Create phases table (IF NOT EXISTS — safe if web UI already ran migration)
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS phases (
  id TEXT PRIMARY KEY NOT NULL,
  board_id TEXT NOT NULL,
  name TEXT NOT NULL,
  color TEXT DEFAULT '#00FFFF' NOT NULL,
  position REAL NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (board_id) REFERENCES boards(id) ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_phases_board_position ON phases(board_id, position);
CREATE UNIQUE INDEX IF NOT EXISTS idx_phases_board_name ON phases(board_id, name);
`)
	if err != nil {
		return fmt.Errorf("migrate v1->v2 create phases: %w", err)
	}

	// Guard: SQLite has no ADD COLUMN IF NOT EXISTS — check pragma_table_info
	var exists int
	db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('cards') WHERE name = 'phase_id'`).Scan(&exists)
	if exists == 0 {
		if _, err := db.Exec(`ALTER TABLE cards ADD COLUMN phase_id TEXT REFERENCES phases(id) ON DELETE SET NULL`); err != nil {
			return fmt.Errorf("migrate v1->v2 alter cards: %w", err)
		}
	}

	if _, err := db.Exec(`UPDATE _meta SET value = '2' WHERE key = 'schema_version'`); err != nil {
		return fmt.Errorf("update schema version: %w", err)
	}
	return nil
}

func migrateV2ToV3(db *sqlx.DB) error {
	// Guard: SQLite has no ADD COLUMN IF NOT EXISTS — check pragma_table_info
	var exists int
	db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('columns') WHERE name = 'description'`).Scan(&exists)
	if exists == 0 {
		if _, err := db.Exec(`ALTER TABLE columns ADD COLUMN description TEXT`); err != nil {
			return fmt.Errorf("migrate v2->v3 alter columns: %w", err)
		}
	}

	if _, err := db.Exec(`UPDATE _meta SET value = '3' WHERE key = 'schema_version'`); err != nil {
		return fmt.Errorf("migrate v2->v3 update schema version: %w", err)
	}

	// Ensure __drizzle_migrations table exists before inserting (ensureDrizzleJournal
	// runs after all migration steps, so we may be here before it runs)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS __drizzle_migrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hash TEXT NOT NULL,
		created_at BIGINT
	)`); err != nil {
		return fmt.Errorf("migrate v2->v3 create drizzle table: %w", err)
	}

	// Mark the column-descriptions Drizzle migration as applied
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM __drizzle_migrations WHERE hash = '0003_overjoyed_reaper'`).Scan(&count)
	if count == 0 {
		if _, err := db.Exec(`INSERT INTO __drizzle_migrations (hash, created_at) VALUES ('0003_overjoyed_reaper', 1776977991688)`); err != nil {
			return fmt.Errorf("migrate v2->v3 drizzle journal: %w", err)
		}
	}
	return nil
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
  description TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_columns_board_position ON columns(board_id, position);

CREATE TABLE IF NOT EXISTS phases (
  id TEXT PRIMARY KEY NOT NULL,
  board_id TEXT NOT NULL,
  name TEXT NOT NULL,
  color TEXT DEFAULT '#00FFFF' NOT NULL,
  position REAL NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (board_id) REFERENCES boards(id) ON UPDATE NO ACTION ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_phases_board_position ON phases(board_id, position);
CREATE UNIQUE INDEX IF NOT EXISTS idx_phases_board_name ON phases(board_id, name);

CREATE TABLE IF NOT EXISTS cards (
  id TEXT PRIMARY KEY,
  column_id TEXT NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT DEFAULT '',
  priority TEXT DEFAULT 'medium' CHECK(priority IN ('low','medium','high','critical')),
  position REAL NOT NULL,
  parent_card_id TEXT,
  due_date TEXT,
  phase_id TEXT REFERENCES phases(id) ON DELETE SET NULL,
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
