package r3sqlite3

import (
	"context"
	"database/sql"
)

// Sqlite3Raw is a database/sql wrapper that allows executing arbitrary queries.
// Is considered to be embedded in Sqlite3CRUD.
type Sqlite3Raw[T any, ID any] struct {
	db   *sql.DB
	meta structMeta
}

// NewSqlite3Raw creates a new Sqlite3Raw instance.
func NewSqlite3Raw[T any, ID comparable](db *sql.DB) *Sqlite3Raw[T, ID] {
	return &Sqlite3Raw[T, ID]{
		db:   db,
		meta: getStructMeta[T](),
	}
}

// Query executes a raw SQL query and scans results into a slice of T.
// The query should use ? placeholders.
func (r *Sqlite3Raw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntities[T](rows, &r.meta)
}

// QueryRow executes a raw SQL query expected to return a single row and scans it into T.
func (r *Sqlite3Raw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	var entity T
	dests := r.meta.scanDest(&entity)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Exec executes a raw SQL query that does not return rows (INSERT, UPDATE, DELETE).
func (r *Sqlite3Raw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

// DB returns the underlying *sql.DB for fully custom usage.
func (r *Sqlite3Raw[T, ID]) DB() *sql.DB { return r.db }
