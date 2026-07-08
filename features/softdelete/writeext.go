package softdelete

import (
	"context"

	"github.com/amberpixels/r3"
)

var (
	_ r3.Upserter[any, any]    = &CRUD[any, any]{}
	_ r3.BulkPatcher[any, any] = &CRUD[any, any]{}
)

// Upsert forwards to the inner CRUD's Upserter capability (soft-delete adds no
// concern to an upsert), returning r3.ErrUpsertNotSupported when the inner repo
// does not implement it.
func (s *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := s.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}
	return up.Upsert(ctx, entity, opts...)
}

// PatchWhere forwards to the inner CRUD's BulkPatcher capability, returning
// r3.ErrBulkPatchNotSupported when the inner repo does not implement it.
func (s *CRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	bp, ok := s.inner.(r3.BulkPatcher[T, ID])
	if !ok {
		return 0, r3.ErrBulkPatchNotSupported
	}
	return bp.PatchWhere(ctx, filters, entity, fields)
}
