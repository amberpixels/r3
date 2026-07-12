package validation

import (
	"context"

	"github.com/amberpixels/r3"
)

var (
	_ r3.Upserter[any, any]    = &CRUD[any, any]{}
	_ r3.BulkPatcher[any, any] = &CRUD[any, any]{}
)

// Upsert validates the full entity (OpUpsert, mirroring Create since an upsert
// carries the whole entity), then delegates to inner.Upsert. Returns
// r3.ErrUpsertNotSupported when the inner repo can't upsert.
func (v *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := v.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}

	if err := v.validator.Validate(ctx, Request[T, ID]{
		Operation: OpUpsert,
		Entity:    entity,
	}); err != nil {
		var zero T
		return zero, err
	}

	return up.Upsert(ctx, entity, opts...)
}

// PatchWhere validates the entity/fields (OpPatchWhere), then delegates to
// inner.PatchWhere. Like Patch, only Fields are meaningful; there is no single
// Existing row, so Existing/Merged stay nil. Returns r3.ErrBulkPatchNotSupported
// when the inner repo can't bulk-patch.
func (v *CRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	bp, ok := v.inner.(r3.BulkPatcher[T, ID])
	if !ok {
		return 0, r3.ErrBulkPatchNotSupported
	}

	if err := v.validator.Validate(ctx, Request[T, ID]{
		Operation: OpPatchWhere,
		Entity:    entity,
		Fields:    fields,
	}); err != nil {
		return 0, err
	}

	return bp.PatchWhere(ctx, filters, entity, fields)
}
