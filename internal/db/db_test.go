package db

import (
	"testing"

	"github.com/jmoiron/sqlx"
)

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
	t.Setenv("CLAUDE_PLUGIN_DATA", "${CLAUDE_PLUGIN_DATA}")
	got := ResolveDbPath()
	// Should fall through to ~/.cyber-mango/kanban.db
	if got == "${CYBER_MANGO_DB_PATH}" || got == "${CLAUDE_PLUGIN_DATA}" {
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

	tables := []string{"boards", "columns", "cards", "tags", "card_tags", "activity_log", "_meta"}
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
