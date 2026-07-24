package transactor

import (
	"context"

	"github.com/amberpixels/r3"
)

// CRUD adds transaction capabilities to the inner repo (BeginTx delegated when it
// implements [r3.Transactor], else [r3.ErrTransactionsNotSupported]); standard
// methods pass through unchanged. See the package doc.
type CRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
}

var (
	_ r3.CRUD[any, any]     = &CRUD[any, any]{}
	_ r3.Aggregator         = &CRUD[any, any]{}
	_ r3.RelationAggregator = &CRUD[any, any]{}
)

// WithTransactor wraps inner with transaction capabilities.
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

// BeginTx starts a transaction if the backend supports it. It walks the chain
// below the transactor to the backend, begins the transaction there, and
// re-applies the same decorator stack on top - so the returned [r3.TxCRUD] is
// fully decorated and in-transaction writes are still validated, audited,
// permission-checked, etc. Returns [r3.ErrTransactionsNotSupported] if no layer
// implements [r3.Transactor].
func (c *CRUD[T, ID]) BeginTx(ctx context.Context) (r3.TxCRUD[T, ID], error) {
	return r3.BeginTxChain(ctx, c.inner)
}

// InTx runs fn inside a transaction (if supported), committing when fn returns
// nil and rolling back on an error or panic. Returns
// [r3.ErrTransactionsNotSupported] if the inner CRUD does not implement
// [r3.Transactor].
func (c *CRUD[T, ID]) InTx(ctx context.Context, fn func(tx r3.CRUD[T, ID]) error) error {
	return r3.InTx(ctx, c.inner, fn)
}

// SupportsTransactions reports whether the backend (seen through any decorators
// below the transactor) supports transactions, and so can use BeginTx / InTx.
func (c *CRUD[T, ID]) SupportsTransactions() bool {
	return r3.SupportsTx(c.inner)
}

// Inner returns the wrapped CRUD (for advanced usage / unwrapping).
func (c *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return c.inner
}

// Unwrap returns the wrapped CRUD so decorator-chain walks (capability
// detection, transaction propagation) can reach the backend.
func (c *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return c.inner
}
