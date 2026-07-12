// Package transactor decorates any r3.CRUD[T, ID] to surface the driver's
// transaction support, transparently satisfying the same interface and passing
// standard methods through to the inner repo.
//
// Transactions in r3 are opt-in: drivers that support them implement
// [r3.Transactor] (BeginTx). The decorator delegates when the inner CRUD
// satisfies it, else returns [r3.ErrTransactionsNotSupported].
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
//	// repo satisfies r3.CRUD[User, int64] - all methods pass through.
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
