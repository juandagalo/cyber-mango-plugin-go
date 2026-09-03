package services

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// Querier is satisfied by *sqlx.DB and *sqlx.Tx. Helpers that take it can run
// inside a transaction, where they must receive the *sqlx.Tx (see sqltx.Run).
type Querier interface {
	sqlx.Ext
	Get(dest interface{}, query string, args ...interface{}) error
	Select(dest interface{}, query string, args ...interface{}) error
	QueryRow(query string, args ...interface{}) *sql.Row
}
