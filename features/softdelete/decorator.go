package softdelete

import (
	"context"

	"github.com/amberpixels/r3"
)

// CRUD is a decorator that wraps any r3.CRUD[T, ID] and adds Restore and
// HardDelete capabilities. All seven standard CRUD methods are passed through
// to the inner implementation unchanged.
//
// Restore and HardDelete are delegated to the inner CRUD if it implements the
// SoftDeleter[ID] interface. Otherwise, ErrNotSoftDeletable is returned.
type CRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
}

// Compile-time check that CRUD satisfies r3.CRUD.
var _ r3.CRUD[any, any] = &CRUD[any, any]{}

// WithSoftDelete wraps an existing r3.CRUD with soft-delete capabilities
// (Restore and HardDelete). All standard CRUD methods pass through unchanged.
func WithSoftDelete[T any, ID comparable](inner r3.CRUD[T, ID]) *CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner}
}

// Create passes through to the inner CRUD.
func (s *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	return s.inner.Create(ctx, entity)
}

// Get passes through to the inner CRUD.
func (s *CRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	return s.inner.Get(ctx, id, qarg...)
}

// List passes through to the inner CRUD.
func (s *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	return s.inner.List(ctx, qarg...)
}

// Count passes through to the inner CRUD.
func (s *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	return s.inner.Count(ctx, qarg...)
}

// Update passes through to the inner CRUD.
func (s *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	return s.inner.Update(ctx, entity)
}

// Patch passes through to the inner CRUD.
func (s *CRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	return s.inner.Patch(ctx, entity, fields)
}

// Delete passes through to the inner CRUD.
func (s *CRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	return s.inner.Delete(ctx, id)
}

// Restore un-deletes a soft-deleted record by clearing its deleted_at field.
// It looks for SoftDeleter support anywhere in the inner decorator chain, so it
// works even when other decorators sit between this one and the backend.
// Returns ErrNotSoftDeletable if no layer implements SoftDeleter.
func (s *CRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	sd, ok := r3.As[SoftDeleter[ID]](s.inner)
	if !ok {
		return ErrNotSoftDeletable
	}
	return sd.Restore(ctx, id)
}

// HardDelete permanently removes a record, bypassing soft-delete.
// It looks for SoftDeleter support anywhere in the inner decorator chain.
// Returns ErrNotSoftDeletable if no layer implements SoftDeleter.
func (s *CRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	sd, ok := r3.As[SoftDeleter[ID]](s.inner)
	if !ok {
		return ErrNotSoftDeletable
	}
	return sd.HardDelete(ctx, id)
}

// Inner returns the wrapped CRUD (for advanced usage / unwrapping).
func (s *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return s.inner
}

// Unwrap returns the wrapped CRUD so capability detection and transaction
// propagation can walk the decorator chain.
func (s *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return s.inner
}

// Rewrap rebuilds this decorator around a different inner CRUD (used to
// re-apply the soft-delete layer on top of a transaction-bound CRUD).
func (s *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return WithSoftDelete[T, ID](inner)
}
