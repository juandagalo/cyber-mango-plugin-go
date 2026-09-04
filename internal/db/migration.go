package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/sqltx"
)

const currentSchemaVersion = "4"

// RunMigrations runs each step (DDL, version stamp, journal row) in its own
// transaction, so a failed step leaves the previous version fully intact.
func RunMigrations(db *sqlx.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _meta (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		return fmt.Errorf("create _meta: %w", err)
	}

	var version string
	err := db.QueryRow(`SELECT value FROM _meta WHERE key = 'schema_version'`).Scan(&version)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read schema version: %w", err)
		}
		if err := sqltx.Run(db, createSchema); err != nil {
			return err
		}
		version = currentSchemaVersion
	}

	if version == "1" {
		if err := sqltx.Run(db, migrateV1ToV2); err != nil {
			return err
		}
		version = "2"
	}

	if version == "2" {
		if err := sqltx.Run(db, migrateV2ToV3); err != nil {
			return err
		}
		version = "3"
	}

	if version == "3" {
		if err := sqltx.Run(db, migrateV3ToV4); err != nil {
			return err
		}
		version = "4"
	}

	if err := sqltx.Run(db, ensureDrizzleJournal); err != nil {
		return fmt.Errorf("drizzle journal: %w", err)
	}

	return nil
}

// ensureDrizzleJournal marks the web UI's Drizzle migrations as applied so it
// does not re-run CREATE TABLE against a schema this plugin already created.
func ensureDrizzleJournal(tx *sqlx.Tx) error {
	if err := ensureJournalTable(tx); err != nil {
		return err
	}
	if err := ensureJournalTag(tx, "0000_wandering_sister_grimm", 1776186662950); err != nil {
		return err
	}
	if err := ensureJournalTag(tx, "0001_right_polaris", 1776299103511); err != nil {
		return err
	}
	return ensureJournalTag(tx, "0002_old_vengeance", 1776799182756)
}

func migrateV1ToV2(tx *sqlx.Tx) error {
	if err := ensurePhasesTable(tx); err != nil {
		return fmt.Errorf("migrate v1->v2: %w", err)
	}
	if err := ensureCardsPhaseID(tx); err != nil {
		return fmt.Errorf("migrate v1->v2: %w", err)
	}
	if _, err := tx.Exec(`UPDATE _meta SET value = '2' WHERE key = 'schema_version'`); err != nil {
		return fmt.Errorf("migrate v1->v2 update schema version: %w", err)
	}
	return nil
}

func migrateV2ToV3(tx *sqlx.Tx) error {
	if err := ensureColumnsDescription(tx); err != nil {
		return fmt.Errorf("migrate v2->v3: %w", err)
	}
	if _, err := tx.Exec(`UPDATE _meta SET value = '3' WHERE key = 'schema_version'`); err != nil {
		return fmt.Errorf("migrate v2->v3 update schema version: %w", err)
	}
	if err := ensureJournalTable(tx); err != nil {
		return fmt.Errorf("migrate v2->v3: %w", err)
	}
	if err := ensureJournalTag(tx, "0003_overjoyed_reaper", 1776977991688); err != nil {
		return fmt.Errorf("migrate v2->v3: %w", err)
	}
	return nil
}

// migrateV3ToV4 is Go-only: the web UI has no matching Drizzle migration, so
// no journal tag is written.
func migrateV3ToV4(tx *sqlx.Tx) error {
	if err := ensureActivityLogIndexes(tx); err != nil {
		return fmt.Errorf("migrate v3->v4: %w", err)
	}
	if _, err := tx.Exec(`UPDATE _meta SET value = '4' WHERE key = 'schema_version'`); err != nil {
		return fmt.Errorf("migrate v3->v4 update schema version: %w", err)
	}
	return nil
}

// createSchema handles both a brand-new file and a Drizzle-first file the web
// UI created at an older shape: CREATE TABLE IF NOT EXISTS is a no-op on the
// latter, so every incremental guard must run before the current version is
// stamped or the missing columns would never be added.
func createSchema(tx *sqlx.Tx) error {
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
	if err := ensurePhasesTable(tx); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := ensureCardsPhaseID(tx); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := ensureColumnsDescription(tx); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := ensureJournalTable(tx); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := ensureJournalTag(tx, "0003_overjoyed_reaper", 1776977991688); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := ensureActivityLogIndexes(tx); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO _meta (key, value) VALUES ('schema_version', ?)`, currentSchemaVersion); err != nil {
		return fmt.Errorf("stamp schema version: %w", err)
	}
	return nil
}

func ensurePhasesTable(tx *sqlx.Tx) error {
	_, err := tx.Exec(`
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
		return fmt.Errorf("create phases: %w", err)
	}
	return nil
}

func ensureCardsPhaseID(tx *sqlx.Tx) error {
	return ensureColumn(tx, "cards", "phase_id", `ALTER TABLE cards ADD COLUMN phase_id TEXT REFERENCES phases(id) ON DELETE SET NULL`)
}

func ensureColumnsDescription(tx *sqlx.Tx) error {
	return ensureColumn(tx, "columns", "description", `ALTER TABLE columns ADD COLUMN description TEXT`)
}

// idx_activity_log_datetime is an expression index: the hooks filter on
// datetime(created_at) (see internal/hooks), and SQLite only uses it when the
// query expression matches this text exactly.
func ensureActivityLogIndexes(tx *sqlx.Tx) error {
	_, err := tx.Exec(`
CREATE INDEX IF NOT EXISTS idx_activity_log_datetime ON activity_log(datetime(created_at));
CREATE INDEX IF NOT EXISTS idx_activity_log_board_created ON activity_log(board_id, created_at);
`)
	if err != nil {
		return fmt.Errorf("create activity_log indexes: %w", err)
	}
	return nil
}

// SQLite has no ADD COLUMN IF NOT EXISTS, so the guard reads pragma_table_info
// first. A failed read is an error, never a silent skip.
func ensureColumn(tx *sqlx.Tx, table, column, alter string) error {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&exists); err != nil {
		return fmt.Errorf("inspect %s.%s: %w", table, column, err)
	}
	if exists == 1 {
		return nil
	}
	if _, err := tx.Exec(alter); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

func ensureJournalTable(tx *sqlx.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS __drizzle_migrations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hash TEXT NOT NULL,
		created_at BIGINT
	)`)
	if err != nil {
		return fmt.Errorf("create drizzle journal: %w", err)
	}
	return nil
}

func ensureJournalTag(tx *sqlx.Tx, hash string, createdAt int64) error {
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM __drizzle_migrations WHERE hash = ?`, hash).Scan(&count); err != nil {
		return fmt.Errorf("inspect drizzle journal %s: %w", hash, err)
	}
	if count > 0 {
		return nil
	}
	if _, err := tx.Exec(`INSERT INTO __drizzle_migrations (hash, created_at) VALUES (?, ?)`, hash, createdAt); err != nil {
		return fmt.Errorf("insert drizzle journal %s: %w", hash, err)
	}
	return nil
}
