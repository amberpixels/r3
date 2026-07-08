package validation

import (
	"context"

	"github.com/amberpixels/r3"
)

// CRUD is a decorator that wraps any r3.CRUD[T, ID] and validates entities
// before mutation operations (Create, Update, Patch). Read operations (Get, List)
// and Delete pass through without validation.
//
// The decorator is stateless -- it doesn't define validation rules. It delegates
// to a user-provided [Validator] implementation that can use any validation library.
//
// It transparently satisfies the r3.CRUD[T, ID] interface, so it can be used
// as a drop-in replacement for any CRUD repository.
//
// Usage:
//
//	repo := validation.WithValidation[Pet, int64](
//	    innerRepo,
//	    myValidator,
//	    validation.WithIDFunc[Pet, int64](func(p Pet) int64 { return p.ID }),
//	)
type CRUD[T any, ID comparable] struct {
	inner     r3.CRUD[T, ID]
	validator Validator[T, ID]
	opts      Options[T, ID]
}

// Compile-time check that CRUD satisfies r3.CRUD.
var _ r3.CRUD[any, any] = &CRUD[any, any]{}
var _ r3.Aggregator = &CRUD[any, any]{}

// WithValidation wraps an existing r3.CRUD with validation.
// The validator parameter defines the validation logic for mutation operations.
//
// Example:
//
//	repo := validation.WithValidation[Pet, int64](
//	    innerRepo,
//	    myValidator,
//	)
func WithValidation[T any, ID comparable](
	inner r3.CRUD[T, ID],
	validator Validator[T, ID],
	optFns ...Option[T, ID],
) *CRUD[T, ID] {
	var opts Options[T, ID]
	for _, fn := range optFns {
		fn(&opts)
	}
	applyDefaults(&opts)

	return &CRUD[T, ID]{
		inner:     inner,
		validator: validator,
		opts:      opts,
	}
}

// Inner returns the underlying CRUD repository (unwrapped).
// Useful when you need to bypass validation for a specific operation.
func (v *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return v.inner
}

// Unwrap returns the wrapped CRUD so capability detection and transaction
// propagation can walk the decorator chain.
func (v *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return v.inner
}

// Rewrap rebuilds this decorator around a different inner CRUD (used to
// re-apply the validation layer on top of a transaction-bound CRUD).
func (v *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner, validator: v.validator, opts: v.opts}
}

// Create validates the entity, then delegates to inner.Create.
func (v *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	if err := v.validator.Validate(ctx, Request[T, ID]{
		Operation: OpCreate,
		Entity:    entity,
	}); err != nil {
		var zero T
		return zero, err
	}

	return v.inner.Create(ctx, entity)
}

// Get delegates directly to inner.Get (no validation on reads).
func (v *CRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	return v.inner.Get(ctx, id, qarg...)
}

// List delegates directly to inner.List (no validation on reads).
func (v *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	return v.inner.List(ctx, qarg...)
}

// Count delegates directly to inner.Count (no validation on reads).
func (v *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	return v.inner.Count(ctx, qarg...)
}

// Aggregate delegates directly to the inner CRUD's Aggregate (no validation
// on reads).
func (v *CRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	agg, ok := v.inner.(r3.Aggregator)
	if !ok {
		return nil, r3.ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, qarg...)
}

// Update optionally fetches the existing entity (if IDFunc is set),
// validates, then delegates to inner.Update.
//
// As with Patch, the fetch-validate-write sequence is atomic only when this
// decorator runs inside a transaction (see the transactor feature); otherwise
// the read-before-validate has a TOCTOU window against concurrent writers.
func (v *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	req := Request[T, ID]{
		Operation: OpUpdate,
		Entity:    entity,
	}

	if v.opts.IDFunc != nil {
		id := v.opts.IDFunc(entity)
		existing, err := v.inner.Get(ctx, id)
		if err != nil {
			var zero T
			return zero, err
		}
		req.Existing = &existing
	}

	if err := v.validator.Validate(ctx, req); err != nil {
		var zero T
		return zero, err
	}

	return v.inner.Update(ctx, entity)
}

// Patch optionally fetches the existing entity (if IDFunc is set),
// validates with the Fields list populated, then delegates to inner.Patch.
//
// When IDFunc is set the request also carries Merged — the patch overlaid on the
// current state — so whole-entity validators don't see the sparse (zeroed) input.
//
// The fetch-validate-write sequence is only atomic when this decorator runs
// inside a transaction: wrap the repo with the transactor feature and call Patch
// within InTx so the Get and the write share one transaction. Outside a
// transaction the read-before-validate is best-effort (a concurrent writer can
// change the row between the Get and the Patch — a TOCTOU window).
func (v *CRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	req := Request[T, ID]{
		Operation: OpPatch,
		Entity:    entity,
		Fields:    fields,
	}

	if v.opts.IDFunc != nil {
		id := v.opts.IDFunc(entity)
		existing, err := v.inner.Get(ctx, id)
		if err != nil {
			var zero T
			return zero, err
		}
		req.Existing = &existing
		merged := mergePatch(existing, entity, fields)
		req.Merged = &merged
	}

	if err := v.validator.Validate(ctx, req); err != nil {
		var zero T
		return zero, err
	}

	return v.inner.Patch(ctx, entity, fields)
}

// Delete delegates directly to inner.Delete (no validation on deletes).
func (v *CRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	return v.inner.Delete(ctx, id)
}
