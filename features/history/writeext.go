package history

import (
	"context"
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
)

var (
	_ r3.Upserter[any, any]    = &CRUD[any, any]{}
	_ r3.BulkPatcher[any, any] = &CRUD[any, any]{}
)

// Upsert inserts-or-updates via the inner CRUD and records the change. To decide
// between a create and an update entry — and to diff the update — it fetches the
// pre-state by primary key (via IDFunc) before the write: a hit records an
// ActionUpdate with a field-level diff, a miss records an ActionCreate.
//
// The pre-state lookup is by PK. When the upsert's conflict target is NOT the PK
// (a custom OnConflict on a unique column), a colliding row may be missed and
// the change recorded as a create — an accepted approximation, since the audited
// KV/settings use case conflicts on its primary key.
func (h *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := h.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}

	var old T
	var hasOld bool
	if h.opts.IDFunc != nil {
		id := h.opts.IDFunc(entity)
		if oldEntity, err := h.inner.Get(ctx, id); err == nil {
			old = oldEntity
			hasOld = true
		}
	}

	result, err := up.Upsert(ctx, entity, opts...)
	if err != nil {
		return result, err
	}

	action := ActionCreate
	diffFn := func() []FieldChange { return DiffCreate(result) }
	if hasOld {
		oldState := old
		action = ActionUpdate
		diffFn = func() []FieldChange { return h.opts.DiffFunc(oldState, result) }
	}

	if recErr := h.record(
		ctx,
		recordInfo[T]{entity: result, old: old, hasOld: hasOld, action: action},
		diffFn,
	); recErr != nil {
		return result, recErr
	}
	return result, nil
}

// PatchWhere runs a bulk conditional update via the inner CRUD and records one
// ActionPatch change per affected row. It snapshots the matching rows before the
// write (a List over the same filters) and re-reads each by PK afterward to diff
// old vs new — so a bulk sweep still leaves a per-row audit trail. This is
// bounded work intended for the small recovery sweeps this capability targets.
//
// Recording per-row history requires IDFunc; without it the change cannot be
// attributed to specific rows, so PatchWhere reports the gap via the error
// handler (loud, never silent) and still performs the update.
func (h *CRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	bp, ok := h.inner.(r3.BulkPatcher[T, ID])
	if !ok {
		return 0, r3.ErrBulkPatchNotSupported
	}

	var oldRows []T
	if h.opts.IDFunc == nil {
		h.handleError(ctx, errors.New(
			"r3history: PatchWhere without IDFunc cannot record per-row history; the bulk update is not audited"))
	} else if rows, _, err := h.inner.List(ctx, r3.Query{Filters: filters, Pagination: r3.Unpaginated()}); err != nil {
		h.handleError(ctx, fmt.Errorf("r3history: PatchWhere pre-list failed; the bulk update is not audited: %w", err))
	} else {
		oldRows = rows
	}

	affected, err := bp.PatchWhere(ctx, filters, entity, fields)
	if err != nil {
		return affected, err
	}

	fieldNames := r3.FieldsToStrings(fields)
	for _, old := range oldRows {
		oldState := old
		newEntity, gErr := h.inner.Get(ctx, h.opts.IDFunc(oldState))
		if gErr != nil {
			h.handleError(ctx, fmt.Errorf(
				"r3history: PatchWhere post-read failed for one row; skipping its audit: %w", gErr))
			continue
		}
		if recErr := h.record(
			ctx,
			recordInfo[T]{entity: newEntity, old: oldState, hasOld: true, action: ActionPatch},
			func() []FieldChange { return DiffWithFields(oldState, newEntity, fieldNames) },
		); recErr != nil {
			return affected, recErr
		}
	}
	return affected, nil
}
