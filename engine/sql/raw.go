package enginesql

import (
	"context"
	"database/sql"
)

// BaseRaw is a database/sql wrapper that allows executing arbitrary queries
// with automatic struct scanning based on StructMeta.
type BaseRaw[T any, ID any] struct {
	// Executor is the SQL executor (either *sql.DB or *sql.Tx).
	Executor SQLExecutor
	Meta     StructMeta
}

// NewBaseRaw creates a new BaseRaw instance.
func NewBaseRaw[T any, ID comparable](executor SQLExecutor, meta StructMeta) *BaseRaw[T, ID] {
	return &BaseRaw[T, ID]{
		Executor: executor,
		Meta:     meta,
	}
}

// Query executes a raw SQL query and scans results into a slice of T.
func (r *BaseRaw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	rows, err := r.Executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanEntities[T](rows, &r.Meta)
}

// QueryRow executes a raw SQL query expected to return a single row and scans it into T.
func (r *BaseRaw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	var entity T
	dests := r.Meta.ScanDest(&entity)
	err := r.Executor.QueryRowContext(ctx, query, args...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Exec executes a raw SQL query that does not return rows (INSERT, UPDATE, DELETE).
func (r *BaseRaw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.Executor.ExecContext(ctx, query, args...)
}
