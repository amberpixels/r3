package r3

import (
	"context"
	"errors"
	"slices"
)

// Transactor is an optional interface that CRUD implementations may satisfy
// to indicate transaction support. Not every backend supports transactions
// (e.g. in-memory stores, YAML files, some NoSQL), so this is opt-in.
//
// Use a type assertion to check whether a repository supports transactions:
//
//	txr, ok := repo.(r3.Transactor[MyEntity, int64])
//	if !ok {
//	    // transactions not supported
//	}
//
// Or use the [InTx] helper for ergonomic usage with automatic commit/rollback.
type Transactor[T any, ID comparable] interface {
	// BeginTx starts a transaction and returns a [TxCRUD] scoped to that transaction.
	// All operations on the returned TxCRUD execute within the transaction.
	// The caller MUST call either Commit or Rollback on the returned TxCRUD.
	BeginTx(ctx context.Context) (TxCRUD[T, ID], error)
}

// TxCRUD is a transactional CRUD that operates within a single transaction.
// It embeds [CRUD] for all standard operations and adds Commit/Rollback
// to control the transaction lifecycle.
//
// After Commit or Rollback is called, the TxCRUD should not be used for
// further operations.
type TxCRUD[T any, ID comparable] interface {
	CRUD[T, ID]

	// Commit commits the transaction.
	// After Commit, the TxCRUD must not be used.
	Commit() error

	// Rollback aborts the transaction.
	// Rollback is a no-op if Commit was already called.
	// After Rollback, the TxCRUD must not be used.
	Rollback() error
}

// ErrTransactionsNotSupported is returned by [InTx] when the repository
// does not implement the [Transactor] interface.
var ErrTransactionsNotSupported = errors.New("r3: transactions not supported by this repository")

// InTx runs fn inside a transaction if the repository supports it.
// It begins a transaction, calls fn with a transactional CRUD, and:
//   - commits if fn returns nil
//   - rolls back if fn returns an error or panics
//
// The repository may be wrapped in decorators (validation, history,
// permissions, ...). InTx walks the decorator chain to the backend
// [Transactor], begins the transaction there, and re-applies the same decorator
// stack on top of the transaction-bound CRUD. This means the CRUD passed to fn
// is fully decorated, so decorated behaviour still runs for writes inside the
// transaction rather than being bypassed.
//
// Returns [ErrTransactionsNotSupported] if no layer in the chain implements
// [Transactor].
//
// Example:
//
//	err := r3.InTx(ctx, userRepo, func(tx r3.CRUD[User, int64]) error {
//	    user, err := tx.Create(ctx, newUser)
//	    if err != nil {
//	        return err
//	    }
//	    return nil
//	})
func InTx[T any, ID comparable](
	ctx context.Context,
	repo CRUD[T, ID],
	fn func(tx CRUD[T, ID]) error,
) error {
	txr, rebuild, ok := resolveTransactor(repo)
	if !ok {
		return ErrTransactionsNotSupported
	}

	baseTx, txErr := txr.BeginTx(ctx)
	if txErr != nil {
		return txErr
	}
	decorated := rebuild(baseTx)

	defer func() {
		if p := recover(); p != nil {
			_ = baseTx.Rollback()
			panic(p) // re-panic after rollback
		}
	}()

	if txErr = fn(decorated); txErr != nil {
		_ = baseTx.Rollback()
		return txErr
	}

	return baseTx.Commit()
}

// BeginTxChain begins a transaction on the backend [Transactor] reached by
// walking repo's decorator chain, and returns a [TxCRUD] whose CRUD methods are
// the same decorator stack re-applied on top of the transaction. Commit and
// Rollback drive the underlying backend transaction.
//
// It is the chain-aware counterpart to calling BeginTx directly on a backend,
// and is what decorators delegate to so transactions compose with decoration.
// Returns [ErrTransactionsNotSupported] if no layer implements [Transactor].
func BeginTxChain[T any, ID comparable](ctx context.Context, repo CRUD[T, ID]) (TxCRUD[T, ID], error) {
	txr, rebuild, ok := resolveTransactor(repo)
	if !ok {
		return nil, ErrTransactionsNotSupported
	}
	baseTx, err := txr.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	return &decoratedTx[T, ID]{CRUD: rebuild(baseTx), base: baseTx}, nil
}

// SupportsTx reports whether repo's decorator chain reaches a backend that
// implements [Transactor]. Unlike a direct type assertion it sees through
// decorators, and unlike [As] it does not treat an intermediate decorator that
// merely exposes BeginTx as proof of support — only the backend counts.
func SupportsTx[T any, ID comparable](repo CRUD[T, ID]) bool {
	_, _, ok := resolveTransactor(repo)
	return ok
}

// resolveTransactor walks repo's decorator chain via [Unwrapper] until it
// reaches the backend (the first layer that is not an Unwrapper). That backend
// must implement [Transactor]. It returns the backend transactor and a rebuild
// function that re-applies every [Rewrapper] decorator seen on the way down, in
// the original outer-to-inner order, on top of a given inner CRUD.
func resolveTransactor[T any, ID comparable](
	repo CRUD[T, ID],
) (Transactor[T, ID], func(CRUD[T, ID]) CRUD[T, ID], bool) {
	var rewrappers []Rewrapper[T, ID]
	cur := repo
	for {
		u, isUnwrapper := any(cur).(Unwrapper[T, ID])
		if !isUnwrapper {
			// Bottom of the chain: the backend. It must be a Transactor.
			txr, ok := any(cur).(Transactor[T, ID])
			if !ok {
				return nil, nil, false
			}
			rebuild := func(inner CRUD[T, ID]) CRUD[T, ID] {
				// Re-apply decorators inner-most first so the rebuilt chain has
				// the same nesting order as the original.
				for _, rw := range slices.Backward(rewrappers) {
					inner = rw.Rewrap(inner)
				}
				return inner
			}
			return txr, rebuild, true
		}
		if rw, ok := any(cur).(Rewrapper[T, ID]); ok {
			rewrappers = append(rewrappers, rw)
		}
		cur = u.Unwrap()
	}
}

// decoratedTx is a [TxCRUD] whose CRUD behaviour is a decorator chain rebuilt on
// top of a backend transaction; Commit/Rollback drive that backend transaction.
type decoratedTx[T any, ID comparable] struct {
	CRUD[T, ID]
	base TxCRUD[T, ID]
}

// Unwrap exposes the decorated chain so capability detection ([As]) can descend
// into a transaction-bound CRUD.
func (d *decoratedTx[T, ID]) Unwrap() CRUD[T, ID] { return d.CRUD }

// Aggregate forwards to the rebuilt decorated chain when it supports
// aggregation, so the capability survives BeginTxChain (the embedded CRUD
// interface would otherwise hide it).
func (d *decoratedTx[T, ID]) Aggregate(ctx context.Context, qarg ...Query) ([]AggregateRow, error) {
	agg, ok := d.CRUD.(Aggregator)
	if !ok {
		return nil, ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, qarg...)
}

func (d *decoratedTx[T, ID]) Commit() error   { return d.base.Commit() }
func (d *decoratedTx[T, ID]) Rollback() error { return d.base.Rollback() }
