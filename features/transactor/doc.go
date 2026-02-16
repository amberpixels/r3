// Package transactor provides a decorator for r3.CRUD[T, ID] that surfaces
// transaction capabilities from the underlying driver.
//
// It follows the same decorator pattern as features/softdelete and
// features/history: the decorator wraps any r3.CRUD[T, ID] and transparently
// satisfies the same interface, passing all six CRUD methods through to the
// inner implementation.
//
// Transaction support in r3 is opt-in. Drivers that support transactions
// implement the [r3.Transactor] interface (BeginTx). The decorator delegates
// to the inner CRUD if it satisfies Transactor; otherwise
// [r3.ErrTransactionsNotSupported] is returned.
//
// # Supported drivers
//
// All SQL-based drivers support transactions out of the box:
//   - r3pgx (PostgreSQL via pgx)
//   - r3pq (PostgreSQL via lib/pq)
//   - r3mysql (MySQL)
//   - r3sqlite3 (SQLite)
//   - r3gorm (GORM)
//   - r3bun (Bun)
//   - r3gopg (go-pg)
//
// MongoDB (r3mongo) does not currently implement Transactor.
//
// # Usage
//
//	inner := r3pgx.NewPgxCRUD[User, int64](db)
//	repo := transactor.WithTransactor[User, int64](inner)
//
//	// repo satisfies r3.CRUD[User, int64] — all methods pass through.
//	// Additionally:
//	repo.SupportsTransactions() // true for SQL drivers
//
//	// Start a transaction manually:
//	tx, err := repo.BeginTx(ctx)
//	if err != nil { ... }
//	defer tx.Rollback()
//	tx.Create(ctx, user)
//	tx.Commit()
//
//	// Or use the convenience helper:
//	err := repo.InTx(ctx, func(tx r3.CRUD[User, int64]) error {
//	    _, err := tx.Create(ctx, user)
//	    return err
//	})
//
// # Composability
//
// The decorator can be stacked with other feature decorators:
//
//	repo := transactor.WithTransactor(
//	    history.WithHistory(
//	        softdelete.WithSoftDelete(inner), store, ...opts),
//	)
package transactor
