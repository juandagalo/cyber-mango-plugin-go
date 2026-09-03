package sqltx_test

import (
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/db"
	"github.com/juandagalo/cyber-mango-plugin-go/internal/sqltx"
)

func newTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	testDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	if _, err := testDB.Exec(`CREATE TABLE t (v INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return testDB
}

func rowCount(t *testing.T, testDB *sqlx.DB) int {
	t.Helper()
	var n int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&n); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}

func TestRun_CommitsOnSuccess(t *testing.T) {
	testDB := newTestDB(t)
	err := sqltx.Run(testDB, func(tx *sqlx.Tx) error {
		_, err := tx.Exec(`INSERT INTO t (v) VALUES (1)`)
		return err
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if n := rowCount(t, testDB); n != 1 {
		t.Errorf("want 1 row after commit, got %d", n)
	}
}

func TestRun_RollsBackOnErrorAndReturnsItUnwrapped(t *testing.T) {
	testDB := newTestDB(t)
	boom := errors.New("NOT_FOUND: boom")
	err := sqltx.Run(testDB, func(tx *sqlx.Tx) error {
		if _, err := tx.Exec(`INSERT INTO t (v) VALUES (1)`); err != nil {
			return err
		}
		return boom
	})
	if err != boom {
		t.Fatalf("want the callback error returned as-is, got %v", err)
	}
	if n := rowCount(t, testDB); n != 0 {
		t.Errorf("want 0 rows after rollback, got %d", n)
	}
}

func TestRun_RollsBackOnPanicAndRepanics(t *testing.T) {
	testDB := newTestDB(t)
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("want panic value %q re-raised, got %v", "boom", r)
		}
		if n := rowCount(t, testDB); n != 0 {
			t.Errorf("want 0 rows after panic rollback, got %d", n)
		}
	}()
	sqltx.Run(testDB, func(tx *sqlx.Tx) error {
		if _, err := tx.Exec(`INSERT INTO t (v) VALUES (1)`); err != nil {
			return err
		}
		panic("boom")
	})
}
