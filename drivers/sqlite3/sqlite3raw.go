package r3sqlite3

import (
	"context"
	"database/sql"

	"github.com/amberpixels/r3/sqlbase"
)

// Sqlite3Raw is a thin wrapper around sqlbase.BaseRaw for backward compatibility.
//
// Deprecated: Use sqlbase.BaseRaw directly via Sqlite3CRUD.Raw().
type Sqlite3Raw[T any, ID any] struct {
	*sqlbase.BaseRaw[T, ID]
}

// NewSqlite3Raw creates a new Sqlite3Raw instance.
func NewSqlite3Raw[T any, ID comparable](db *sql.DB) *Sqlite3Raw[T, ID] {
	meta := sqlbase.GetStructMeta[T]()
	return &Sqlite3Raw[T, ID]{
		BaseRaw: sqlbase.NewBaseRaw[T, ID](db, meta),
	}
}

// Query executes a raw SQL query and scans results into a slice of T.
func (r *Sqlite3Raw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	return r.BaseRaw.Query(ctx, query, args...)
}

// QueryRow executes a raw SQL query expected to return a single row and scans it into T.
func (r *Sqlite3Raw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	return r.BaseRaw.QueryRow(ctx, query, args...)
}

// Exec executes a raw SQL query that does not return rows.
func (r *Sqlite3Raw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.BaseRaw.Exec(ctx, query, args...)
}

// DB returns the underlying *sql.DB for fully custom usage.
func (r *Sqlite3Raw[T, ID]) DB() *sql.DB { return r.BaseRaw.Executor.(*sql.DB) }
