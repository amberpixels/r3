package transactor

import (
	"context"

	"github.com/amberpixels/r3"
)

// CRUD is a decorator that wraps any r3.CRUD[T, ID] and adds transaction
// capabilities. All six standard CRUD methods are passed through to the inner
// implementation unchanged.
//
// Transaction operations (BeginTx) are delegated to the inner CRUD if it
// implements the [r3.Transactor] interface. Otherwise,
// [r3.ErrTransactionsNotSupported] is returned.
type CRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
}

// Compile-time check that CRUD satisfies r3.CRUD.
var _ r3.CRUD[any, any] = &CRUD[any, any]{}
var _ r3.Aggregator = &CRUD[any, any]{}
var _ r3.RelationAggregator = &CRUD[any, any]{}

// WithTransactor wraps an existing r3.CRUD with transaction capabilities.
// All standard CRUD methods pass through unchanged.
func WithTransactor[T any, ID comparable](inner r3.CRUD[T, ID]) *CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner}
}

// Create passes through to the inner CRUD.
func (c *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	return c.inner.Create(ctx, entity)
}

// Get passes through to the inner CRUD.
func (c *CRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	return c.inner.Get(ctx, id, qarg...)
}

// List passes through to the inner CRUD.
func (c *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	return c.inner.List(ctx, qarg...)
}

// Count passes through to the inner CRUD.
func (c *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	return c.inner.Count(ctx, qarg...)
}

// Aggregate passes through to the inner CRUD's Aggregate.
func (c *CRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	agg, ok := c.inner.(r3.Aggregator)
	if !ok {
		return nil, r3.ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, qarg...)
}

// AggregateThroughRelation passes through to the inner CRUD.
func (c *CRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	agg, ok := c.inner.(r3.RelationAggregator)
	if !ok {
		return nil, r3.ErrRelationAggregateNotSupported
	}
	return agg.AggregateThroughRelation(ctx, relation, qarg...)
}

// Update passes through to the inner CRUD.
func (c *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	return c.inner.Update(ctx, entity)
}

// Patch passes through to the inner CRUD.
func (c *CRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	return c.inner.Patch(ctx, entity, fields)
}

// Delete passes through to the inner CRUD.
func (c *CRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	return c.inner.Delete(ctx, id)
}

// BeginTx starts a transaction if the underlying backend supports it.
//
// It walks the decorator chain below the transactor to the backend, begins the
// transaction there, and re-applies the same decorator stack on top of it — so
// the returned [r3.TxCRUD] is fully decorated and writes inside the transaction
// are still validated, audited, permission-checked, etc.
//
// Returns [r3.ErrTransactionsNotSupported] if no layer in the chain implements
// [r3.Transactor].
func (c *CRUD[T, ID]) BeginTx(ctx context.Context) (r3.TxCRUD[T, ID], error) {
	return r3.BeginTxChain(ctx, c.inner)
}

// InTx runs fn inside a transaction if the inner CRUD supports it.
// It begins a transaction, calls fn with a transactional CRUD, and:
//   - commits if fn returns nil
//   - rolls back if fn returns an error or panics
//
// Returns [r3.ErrTransactionsNotSupported] if the inner CRUD does not
// implement [r3.Transactor].
func (c *CRUD[T, ID]) InTx(ctx context.Context, fn func(tx r3.CRUD[T, ID]) error) error {
	return r3.InTx(ctx, c.inner, fn)
}

// SupportsTransactions reports whether the underlying backend supports
// transactions (seen through any decorators between the transactor and the
// backend), and can therefore be used with BeginTx / InTx.
func (c *CRUD[T, ID]) SupportsTransactions() bool {
	return r3.SupportsTx(c.inner)
}

// Inner returns the wrapped CRUD (for advanced usage / unwrapping).
func (c *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return c.inner
}

// Unwrap returns the wrapped CRUD so capability detection and transaction
// propagation can walk the decorator chain.
func (c *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return c.inner
}
