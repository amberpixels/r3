package softdelete

import (
	"context"

	"github.com/amberpixels/r3"
)

// CRUD adds Restore and HardDelete to the inner repo (delegated when it
// implements SoftDeleter[ID], else ErrNotSoftDeletable); standard methods pass
// through unchanged. See the package doc.
type CRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
}

var _ r3.CRUD[any, any] = &CRUD[any, any]{}
var _ r3.Aggregator = &CRUD[any, any]{}
var _ r3.RelationAggregator = &CRUD[any, any]{}

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

// Aggregate passes through to the inner CRUD's Aggregate (trashed filtering is
// an engine concern, like List/Count).
func (s *CRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	agg, ok := s.inner.(r3.Aggregator)
	if !ok {
		return nil, r3.ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, qarg...)
}

// AggregateThroughRelation passes through to the inner CRUD (trashed filtering is
// an engine concern, like List/Count).
func (s *CRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	agg, ok := s.inner.(r3.RelationAggregator)
	if !ok {
		return nil, r3.ErrRelationAggregateNotSupported
	}
	return agg.AggregateThroughRelation(ctx, relation, qarg...)
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

// Restore un-deletes a soft-deleted record by clearing its deleted_at field. It
// finds SoftDeleter anywhere in the inner chain, so it works even with other
// decorators between here and the backend. Returns ErrNotSoftDeletable if no
// layer implements it.
func (s *CRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	sd, ok := r3.As[SoftDeleter[ID]](s.inner)
	if !ok {
		return ErrNotSoftDeletable
	}
	return sd.Restore(ctx, id)
}

// HardDelete permanently removes a record, bypassing soft-delete. It finds
// SoftDeleter anywhere in the inner chain. Returns ErrNotSoftDeletable if no
// layer implements it.
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

// Unwrap returns the wrapped CRUD so decorator-chain walks (capability
// detection, transaction propagation) can reach the backend.
func (s *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return s.inner
}

// Rewrap rebuilds this decorator around inner, re-applying the soft-delete layer
// on top of a transaction-bound CRUD.
func (s *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return WithSoftDelete[T, ID](inner)
}
