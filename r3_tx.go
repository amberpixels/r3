package r3

import (
	"context"
	"errors"
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
// Returns [ErrTransactionsNotSupported] if the repository does not implement [Transactor].
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
	txr, ok := repo.(Transactor[T, ID])
	if !ok {
		return ErrTransactionsNotSupported
	}

	txCrud, txErr := txr.BeginTx(ctx)
	if txErr != nil {
		return txErr
	}

	defer func() {
		if p := recover(); p != nil {
			_ = txCrud.Rollback()
			panic(p) // re-panic after rollback
		}
	}()

	if txErr = fn(txCrud); txErr != nil {
		_ = txCrud.Rollback()
		return txErr
	}

	return txCrud.Commit()
}
