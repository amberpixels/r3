package i18n

import (
	"context"

	"github.com/amberpixels/r3"
)

var (
	_ r3.Upserter[struct{}, int64]    = &CRUD[struct{}, int64]{}
	_ r3.BulkPatcher[struct{}, int64] = &CRUD[struct{}, int64]{}
)

// Upsert inserts-or-updates via the inner CRUD, then marks translations of
// changed source text stale - the same best-effort tracking as Update/Patch.
// Returns r3.ErrUpsertNotSupported when the inner repo can't upsert.
func (c *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := c.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}
	result, err := up.Upsert(ctx, entity, opts...)
	if err != nil {
		return result, err
	}
	c.markStale(ctx, result)
	return result, nil
}

// PatchWhere forwards a bulk conditional update to the inner CRUD. It does NOT
// mark translations stale: a bulk update has no per-row result to hash, and this
// capability targets non-translatable status-style columns. If a bulk update can
// touch a translated source field, mark the affected translations stale explicitly
// via Translations().
func (c *CRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	bp, ok := c.inner.(r3.BulkPatcher[T, ID])
	if !ok {
		return 0, r3.ErrBulkPatchNotSupported
	}
	return bp.PatchWhere(ctx, filters, entity, fields)
}
