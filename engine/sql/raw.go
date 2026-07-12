package enginesql

import (
	"context"
	"database/sql"
)

// BaseRaw executes arbitrary SQL with automatic StructMeta-based struct scanning.
type BaseRaw[T any, ID any] struct {
	Executor SQLExecutor // *sql.DB or *sql.Tx
	Meta     StructMeta
}

// NewBaseRaw creates a BaseRaw.
func NewBaseRaw[T any, ID comparable](executor SQLExecutor, meta StructMeta) *BaseRaw[T, ID] {
	return &BaseRaw[T, ID]{
		Executor: executor,
		Meta:     meta,
	}
}

// Query runs a raw query and scans the rows into a slice of T.
func (r *BaseRaw[T, ID]) Query(ctx context.Context, query string, args ...any) ([]T, error) {
	rows, err := r.Executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanEntities[T](rows, &r.Meta)
}

// QueryRow runs a raw single-row query and scans it into T.
func (r *BaseRaw[T, ID]) QueryRow(ctx context.Context, query string, args ...any) (T, error) {
	var entity T
	dests := r.Meta.ScanDest(&entity)
	err := r.Executor.QueryRowContext(ctx, query, args...).Scan(dests...)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// Exec runs a raw non-returning query (INSERT, UPDATE, DELETE).
func (r *BaseRaw[T, ID]) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.Executor.ExecContext(ctx, query, args...)
}
