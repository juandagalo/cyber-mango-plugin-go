package db

import (
	"testing"

	"github.com/jmoiron/sqlx"
)

// helper: build a "v2" DB manually — creates schema at v2 (no description column on columns)
// so we can test the v2->v3 migration path.
func newTestDBAtV2(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create _meta first
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _meta (key TEXT PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatalf("create _meta: %v", err)
	}

	// Create the full schema manually at v2 (columns table WITHOUT description)
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
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create v2 schema: %v", err)
	}

	// Set version to "2"
	if _, err := db.Exec(`INSERT INTO _meta (key, value) VALUES ('schema_version', '2')`); err != nil {
		t.Fatalf("set schema version: %v", err)
	}

	return db
}

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

func TestResolveDbPath_EnvVar(t *testing.T) {
	t.Setenv("CYBER_MANGO_DB_PATH", "/tmp/test.db")
	got := ResolveDbPath()
	if got != "/tmp/test.db" {
		t.Errorf("got %q, want %q", got, "/tmp/test.db")
	}
}

func TestResolveDbPath_UnexpandedTemplate(t *testing.T) {
	t.Setenv("CYBER_MANGO_DB_PATH", "${CYBER_MANGO_DB_PATH}")
	got := ResolveDbPath()
	// Should fall through to ~/.cyber-mango/kanban.db
	if got == "${CYBER_MANGO_DB_PATH}" {
		t.Errorf("unexpanded template not rejected: %q", got)
	}
}

func TestRunMigrations_CreatesAllTables(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	tables := []string{"boards", "columns", "cards", "tags", "card_tags", "activity_log", "phases", "_meta"}
	for _, tbl := range tables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", tbl, err)
		}
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Run twice — should not error
	if err := RunMigrations(db); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}
}

func TestSeedDefaultBoard(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	if err := SeedDefaultBoard(db); err != nil {
		t.Fatalf("SeedDefaultBoard: %v", err)
	}

	var boardCount int
	db.QueryRow(`SELECT COUNT(*) FROM boards`).Scan(&boardCount)
	if boardCount != 1 {
		t.Errorf("want 1 board, got %d", boardCount)
	}

	var colCount int
	db.QueryRow(`SELECT COUNT(*) FROM columns`).Scan(&colCount)
	if colCount != 5 {
		t.Errorf("want 5 columns, got %d", colCount)
	}

	var phaseCount int
	db.QueryRow(`SELECT COUNT(*) FROM phases`).Scan(&phaseCount)
	if phaseCount != 5 {
		t.Errorf("want 5 phases, got %d", phaseCount)
	}
}

func TestSeedDefaultBoard_Idempotent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	// Seed twice — should not create duplicates
	if err := SeedDefaultBoard(db); err != nil {
		t.Fatal(err)
	}
	if err := SeedDefaultBoard(db); err != nil {
		t.Fatal(err)
	}

	var count int
	db.QueryRow(`SELECT COUNT(*) FROM boards`).Scan(&count)
	if count != 1 {
		t.Errorf("want 1 board after double seed, got %d", count)
	}
}

// TestMigration_V2ToV3_AddsDescriptionColumn verifies that running migrations on a v2 DB
// adds the description column to the columns table.
func TestMigration_V2ToV3_AddsDescriptionColumn(t *testing.T) {
	db := newTestDBAtV2(t)

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations on v2 db: %v", err)
	}

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('columns') WHERE name = 'description'`).Scan(&count)
	if err != nil {
		t.Fatalf("query pragma_table_info: %v", err)
	}
	if count != 1 {
		t.Errorf("want description column on columns table, got count=%d", count)
	}
}

// TestMigration_V2ToV3_ExistingColumnsGetNullDescription verifies that rows inserted before
// migration have NULL description after the migration runs.
func TestMigration_V2ToV3_ExistingColumnsGetNullDescription(t *testing.T) {
	db := newTestDBAtV2(t)

	// Insert a board and a column before migration
	if _, err := db.Exec(`INSERT INTO boards (id, name, created_at, updated_at) VALUES ('b1', 'Test Board', datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert board: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO columns (id, board_id, name, position, created_at, updated_at) VALUES ('c1', 'b1', 'Backlog', 1000, datetime('now'), datetime('now'))`); err != nil {
		t.Fatalf("insert column: %v", err)
	}

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations on v2 db: %v", err)
	}

	var desc *string
	err := db.QueryRow(`SELECT description FROM columns WHERE id = 'c1'`).Scan(&desc)
	if err != nil {
		t.Fatalf("query description: %v", err)
	}
	if desc != nil {
		t.Errorf("want NULL description for pre-migration column, got %q", *desc)
	}
}

// TestMigration_V2ToV3_Idempotent verifies running migrations twice on a v2 DB does not error.
func TestMigration_V2ToV3_Idempotent(t *testing.T) {
	db := newTestDBAtV2(t)

	if err := RunMigrations(db); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second RunMigrations (idempotent check): %v", err)
	}
}

// TestMigration_V2ToV3_DrizzleJournalEntry verifies the 0003_overjoyed_reaper entry
// is inserted into __drizzle_migrations after the v2->v3 migration.
func TestMigration_V2ToV3_DrizzleJournalEntry(t *testing.T) {
	db := newTestDBAtV2(t)

	if err := RunMigrations(db); err != nil {
		t.Fatalf("RunMigrations on v2 db: %v", err)
	}

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM __drizzle_migrations WHERE hash = '0003_overjoyed_reaper'`).Scan(&count)
	if err != nil {
		t.Fatalf("query __drizzle_migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("want 1 drizzle journal entry for 0003_overjoyed_reaper, got %d", count)
	}
}
