package r3mysql

import (
	"context"
	"database/sql"

	enginesql "github.com/amberpixels/r3/engine/sql"
)

// MysqlRaw wraps enginesql.BaseRaw.
//
// Deprecated: use enginesql.BaseRaw directly via MysqlCRUD.Raw().
type MysqlRaw[T any, ID any] struct {
	*enginesql.BaseRaw[T, ID]
}

// NewMysqlRaw creates a new MysqlRaw instance.
func NewMysqlRaw[T any, ID comparable](db *sql.DB) *MysqlRaw[T, ID] {
	meta := enginesql.GetStructMeta[T]()
	return &MysqlRaw[T, ID]{
		BaseRaw: enginesql.NewBaseRaw[T, ID](db, meta),
	}
}

// Query executes a raw SQL query and scans results into a slice of T.
func (r *MysqlRaw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	return r.BaseRaw.Query(ctx, query, args...)
}

// QueryRow executes a raw SQL query expected to return a single row and scans it into T.
func (r *MysqlRaw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	return r.BaseRaw.QueryRow(ctx, query, args...)
}

// Exec executes a raw SQL query that does not return rows.
func (r *MysqlRaw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.BaseRaw.Exec(ctx, query, args...)
}

// DB returns the underlying *sql.DB for fully custom usage.
func (r *MysqlRaw[T, ID]) DB() *sql.DB { return r.BaseRaw.Executor.(*sql.DB) } //nolint:errcheck // type is guaranteed by constructor
