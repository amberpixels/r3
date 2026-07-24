package r3

import (
	"context"
	"errors"
	"slices"
)

// Transactor is the opt-in transaction capability a CRUD may satisfy; many
// backends (in-memory, YAML files, some NoSQL) do not. Type-assert to detect
// support, or use [InTx] for automatic commit/rollback.
type Transactor[T any, ID comparable] interface {
	// BeginTx starts a transaction and returns a [TxCRUD] scoped to it. The
	// caller MUST call either Commit or Rollback on it.
	BeginTx(ctx context.Context) (TxCRUD[T, ID], error)
}

// TxCRUD is a [CRUD] bound to a single transaction, plus Commit/Rollback. It
// must not be used after either is called.
type TxCRUD[T any, ID comparable] interface {
	CRUD[T, ID]

	// Commit commits the transaction; the TxCRUD must not be used afterward.
	Commit() error

	// Rollback aborts the transaction (a no-op if Commit already ran); the
	// TxCRUD must not be used afterward.
	Rollback() error
}

// ErrTransactionsNotSupported is returned when no layer in a repo's chain
// implements [Transactor].
var ErrTransactionsNotSupported = errors.New("r3: transactions not supported by this repository")

// InTx runs fn inside a transaction, committing if it returns nil and rolling
// back on error or panic. It walks repo's decorator chain to the backend
// [Transactor], begins there, and re-applies the same decorator stack on top of
// the tx-bound CRUD - so the CRUD passed to fn stays fully decorated and writes
// inside the transaction still run every decorator. Returns
// [ErrTransactionsNotSupported] if no layer in the chain implements [Transactor].
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

// BeginTxChain is the chain-aware counterpart to [Transactor.BeginTx]: it walks
// repo's decorator chain to the backend, begins there, and returns a [TxCRUD]
// with the same decorator stack re-applied on top. Decorators delegate to it so
// transactions compose with decoration. Returns [ErrTransactionsNotSupported]
// if no layer implements [Transactor].
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

// SupportsTx reports whether repo's chain reaches a backend implementing
// [Transactor]. It sees through decorators, and unlike [As] only the backend
// counts - an intermediate decorator merely exposing BeginTx is not proof.
func SupportsTx[T any, ID comparable](repo CRUD[T, ID]) bool {
	_, _, ok := resolveTransactor(repo)
	return ok
}

// resolveTransactor walks repo's chain via [Unwrapper] to the backend (first
// non-Unwrapper), which must be a [Transactor]. It returns that transactor and
// a rebuild func that re-applies every [Rewrapper] seen, preserving the original
// outer-to-inner nesting, on top of a given inner CRUD.
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
				// Re-apply inner-most first to preserve the original nesting.
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
