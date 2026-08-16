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

// Upsert inserts-or-updates via the inner CRUD and records the change. It
// fetches the pre-state before the write: a hit records an ActionUpdate with a
// field-level diff, a miss records an ActionCreate.
//
// The pre-state is looked up through the SAME target the write conflicts on -
// [r3.OnConflict]'s columns when it declares any, the PK otherwise - so an upsert
// that updates an existing row by unique column records the update it performed
// rather than a create.
func (h *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := h.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}

	old, hasOld := h.upsertPreState(ctx, entity, r3.NewUpsertSpec(opts...))

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

// upsertPreState reads the row an Upsert is about to collide with, so the record
// can be an update with a real diff instead of a create.
//
// It resolves the row the same way the write will: by the [r3.OnConflict]
// columns when the spec names any, by primary key otherwise. A conflict column
// that does not map to a struct field falls back to the PK lookup rather than
// querying on a value it could not read, as does a lookup that fails outright -
// a backend error is not evidence that no row exists, and treating it as one
// would log an update as a create. Finding no row, on the other hand, is the
// normal insert case.
func (h *CRUD[T, ID]) upsertPreState(ctx context.Context, entity T, spec r3.UpsertSpec) (T, bool) {
	var zero T

	if len(spec.ConflictColumns) > 0 {
		if values, ok := fieldValuesByColumn(entity, spec.ConflictColumns); ok {
			filters := make(r3.Filters, 0, len(spec.ConflictColumns))
			for i, col := range spec.ConflictColumns {
				filters = append(filters, r3.Eq(col, values[i]))
			}
			existing, _, err := h.inner.List(ctx, r3.Query{
				Filters:    filters,
				Pagination: r3.NewPaginationSpec(1, 1),
			})
			if err == nil {
				if len(existing) > 0 {
					return existing[0], true
				}
				return zero, false
			}
		}
	}

	if h.opts.IDFunc == nil {
		return zero, false
	}
	if oldEntity, err := h.inner.Get(ctx, h.opts.IDFunc(entity)); err == nil {
		return oldEntity, true
	}
	return zero, false
}

// PatchWhere runs a bulk conditional update and records one ActionPatch per
// affected row: it snapshots matching rows before the write (List over the same
// filters) and re-reads each by PK afterward to diff old vs new, so a bulk sweep
// still leaves a per-row trail. Bounded work, for the small recovery sweeps this
// targets.
//
// Per-row history requires IDFunc; without it the change cannot be attributed to
// specific rows, so PatchWhere reports the gap via the error handler (loud, never
// silent) and still performs the update.
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
