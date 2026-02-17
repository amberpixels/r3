package r3pgx

import (
	"context"
	"database/sql"

	enginesql "github.com/amberpixels/r3/engine/sql"
)

// PgxRaw is a thin wrapper around enginesql.BaseRaw for backward compatibility.
// It provides the same Query/QueryRow/Exec methods.
//
// Deprecated: Use enginesql.BaseRaw directly via PgxCRUD.Raw().
type PgxRaw[T any, ID any] struct {
	*enginesql.BaseRaw[T, ID]
}

// NewPgxRaw creates a new PgxRaw instance.
func NewPgxRaw[T any, ID comparable](db *sql.DB) *PgxRaw[T, ID] {
	meta := enginesql.GetStructMeta[T]()
	return &PgxRaw[T, ID]{
		BaseRaw: enginesql.NewBaseRaw[T, ID](db, meta),
	}
}

// Query executes a raw SQL query and scans results into a slice of T.
// The query should use $1, $2, ... PostgreSQL-style placeholders.
func (r *PgxRaw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	return r.BaseRaw.Query(ctx, query, args...)
}

// QueryRow executes a raw SQL query expected to return a single row and scans it into T.
func (r *PgxRaw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	return r.BaseRaw.QueryRow(ctx, query, args...)
}

// Exec executes a raw SQL query that does not return rows (INSERT, UPDATE, DELETE).
func (r *PgxRaw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.BaseRaw.Exec(ctx, query, args...)
}

// DB returns the underlying *sql.DB for fully custom usage.
func (r *PgxRaw[T, ID]) DB() *sql.DB { return r.BaseRaw.Executor.(*sql.DB) } //nolint:errcheck // type is guaranteed by constructor
