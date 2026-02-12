package r3pgx

import (
	"context"
	"database/sql"
)

// PgxRaw is a database/sql wrapper that allows executing arbitrary queries.
// Is considered to be embedded in PgxCRUD.
type PgxRaw[T any, ID any] struct {
	db   *sql.DB
	meta structMeta
}

// NewPgxRaw creates a new PgxRaw instance.
func NewPgxRaw[T any, ID comparable](db *sql.DB) *PgxRaw[T, ID] {
	return &PgxRaw[T, ID]{
		db:   db,
		meta: getStructMeta[T](),
	}
}

// Query executes a raw SQL query and scans results into a slice of T.
// The query should use $1, $2, ... PostgreSQL-style placeholders.
func (r *PgxRaw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntities[T](rows, &r.meta)
}

// QueryRow executes a raw SQL query expected to return a single row and scans it into T.
func (r *PgxRaw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	var entity T
	dests := r.meta.scanDest(&entity)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Exec executes a raw SQL query that does not return rows (INSERT, UPDATE, DELETE).
func (r *PgxRaw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

// DB returns the underlying *sql.DB for fully custom usage.
func (r *PgxRaw[T, ID]) DB() *sql.DB { return r.db }
