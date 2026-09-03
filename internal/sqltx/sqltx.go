package sqltx

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Run commits when fn returns nil and rolls back when fn returns an error
// (returned unchanged) or panics (re-raised after the rollback).
//
// The pool holds a single connection (db.Open sets SetMaxOpenConns(1)), so
// every statement inside fn must go through tx; any call on the *sqlx.DB
// while the transaction is open blocks forever.
func Run(db *sqlx.DB, fn func(tx *sqlx.Tx) error) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
