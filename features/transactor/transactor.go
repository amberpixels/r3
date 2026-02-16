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

// BeginTx starts a transaction if the inner CRUD supports it.
// Returns [r3.ErrTransactionsNotSupported] if the inner CRUD does not
// implement [r3.Transactor].
func (c *CRUD[T, ID]) BeginTx(ctx context.Context) (r3.TxCRUD[T, ID], error) {
	txr, ok := c.inner.(r3.Transactor[T, ID])
	if !ok {
		return nil, r3.ErrTransactionsNotSupported
	}
	return txr.BeginTx(ctx)
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

// SupportsTransactions reports whether the inner CRUD implements
// [r3.Transactor] and can be used with BeginTx / InTx.
func (c *CRUD[T, ID]) SupportsTransactions() bool {
	_, ok := c.inner.(r3.Transactor[T, ID])
	return ok
}

// Inner returns the wrapped CRUD (for advanced usage / unwrapping).
func (c *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return c.inner
}
