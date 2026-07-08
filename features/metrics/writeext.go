package metrics

import (
	"context"
	"time"

	"github.com/amberpixels/r3"
)

var (
	_ r3.Upserter[any, any]    = &CRUD[any, any]{}
	_ r3.BulkPatcher[any, any] = &CRUD[any, any]{}
)

// Upsert inserts-or-updates via the inner CRUD and records metrics (OpUpsert).
// The pre-state is fetched (when IDFunc is set) so diff-based collectors see the
// old entity, mirroring Update. Returns r3.ErrUpsertNotSupported when the inner
// repo has no upsert capability.
func (m *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := m.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}

	var old T
	var hasOld bool
	if m.opts.IDFunc != nil {
		id := m.opts.IDFunc(entity)
		if oldEntity, err := m.inner.Get(ctx, id); err == nil {
			old = oldEntity
			hasOld = true
		}
	}

	start := time.Now()
	result, err := up.Upsert(ctx, entity, opts...)
	duration := time.Since(start)

	opCtx := OperationContext[T, ID]{
		Operation: OpUpsert,
		Duration:  duration,
		Entity:    result,
		OldEntity: old,
		HasOld:    hasOld,
		Err:       err,
	}
	if err == nil && m.opts.IDFunc != nil {
		opCtx.EntityID = m.opts.IDFunc(result)
	}
	m.record(ctx, opCtx)

	return result, err
}

// PatchWhere runs a bulk conditional update via the inner CRUD and records
// metrics (OpPatchWhere, with TotalCount = affected-row count). Returns
// r3.ErrBulkPatchNotSupported when the inner repo has no bulk-patch capability.
func (m *CRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	bp, ok := m.inner.(r3.BulkPatcher[T, ID])
	if !ok {
		return 0, r3.ErrBulkPatchNotSupported
	}

	start := time.Now()
	affected, err := bp.PatchWhere(ctx, filters, entity, fields)
	duration := time.Since(start)

	m.record(ctx, OperationContext[T, ID]{
		Operation:  OpPatchWhere,
		Duration:   duration,
		Entity:     entity,
		Fields:     fields,
		TotalCount: affected,
		Err:        err,
	})

	return affected, err
}
