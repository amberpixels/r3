package r3pq

import (
	"context"
	"database/sql"
)

// PqRaw is a database/sql wrapper that allows executing arbitrary queries.
// Is considered to be embedded in PqCRUD.
type PqRaw[T any, ID any] struct {
	db   *sql.DB
	meta structMeta
}

// NewPqRaw creates a new PqRaw instance.
func NewPqRaw[T any, ID comparable](db *sql.DB) *PqRaw[T, ID] {
	return &PqRaw[T, ID]{
		db:   db,
		meta: getStructMeta[T](),
	}
}

// Query executes a raw SQL query and scans results into a slice of T.
// The query should use $1, $2, ... PostgreSQL-style placeholders.
func (r *PqRaw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEntities[T](rows, &r.meta)
}

// QueryRow executes a raw SQL query expected to return a single row and scans it into T.
func (r *PqRaw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	var entity T
	dests := r.meta.scanDest(&entity)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Exec executes a raw SQL query that does not return rows (INSERT, UPDATE, DELETE).
func (r *PqRaw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

// DB returns the underlying *sql.DB for fully custom usage.
func (r *PqRaw[T, ID]) DB() *sql.DB { return r.db }
