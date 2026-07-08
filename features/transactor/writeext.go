package transactor

import (
	"context"

	"github.com/amberpixels/r3"
)

var (
	_ r3.Upserter[any, any]    = &CRUD[any, any]{}
	_ r3.BulkPatcher[any, any] = &CRUD[any, any]{}
)

// Upsert passes through to the inner CRUD's Upserter capability, so an upsert
// run inside InTx participates in the transaction like any other write.
func (c *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := c.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}
	return up.Upsert(ctx, entity, opts...)
}

// PatchWhere passes through to the inner CRUD's BulkPatcher capability.
func (c *CRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	bp, ok := c.inner.(r3.BulkPatcher[T, ID])
	if !ok {
		return 0, r3.ErrBulkPatchNotSupported
	}
	return bp.PatchWhere(ctx, filters, entity, fields)
}
