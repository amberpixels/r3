package r3bun

import (
	"context"

	"github.com/uptrace/bun"
)

// BunRaw is a Bun wrapper that allows calling any Bun query.
// Is considered to be embedded in BunCRUD.
type BunRaw[T any, ID any] struct {
	db *bun.DB
}

// NewBunRaw creates a new BunRaw instance.
func NewBunRaw[T any, ID comparable](db *bun.DB) *BunRaw[T, ID] {
	return &BunRaw[T, ID]{
		db: db,
	}
}

// Find executes a custom select query and returns the results.
// The callback receives a *bun.SelectQuery to build a custom query.
func (r *BunRaw[T, ID]) Find(ctx context.Context, cb func(*bun.SelectQuery) *bun.SelectQuery) ([]T, error) {
	var entities []T
	query := r.db.NewSelect().Model(&entities)
	query = cb(query)
	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	return entities, nil
}

// Scan executes a custom select query and scans results into the provided destination.
func (r *BunRaw[T, ID]) Scan(ctx context.Context, cb func(*bun.SelectQuery) *bun.SelectQuery, dest any) error {
	var entities []T
	query := r.db.NewSelect().Model(&entities)
	query = cb(query)
	return query.Scan(ctx, dest)
}
