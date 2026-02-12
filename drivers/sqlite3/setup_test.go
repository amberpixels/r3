package r3sqlite3_test

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3" // SQLite3 driver for database/sql
)

func setupSQLiteDB() (*sql.DB, error) {
	// Open an in-memory SQLite database.
	// Each connection gets its own database, so we use a shared cache
	// to allow multiple connections to the same in-memory DB.
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
