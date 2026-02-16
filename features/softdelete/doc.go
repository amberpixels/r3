// Package softdelete provides a decorator for r3.CRUD[T, ID] that adds
// Restore and HardDelete capabilities to any CRUD implementation that
// supports soft-delete.
//
// It follows the same decorator pattern as features/history: the decorator
// wraps any r3.CRUD[T, ID] and transparently satisfies the same interface,
// passing all seven CRUD methods through to the inner implementation.
//
// Soft-delete filtering and Delete-as-UPDATE logic remain in the engines
// (engine/sql, engine/mongo) and ORM drivers (gorm, bun, gopg). The decorator
// only adds new capabilities:
//
//   - Restore: un-deletes a soft-deleted record by clearing its deleted_at field.
//   - HardDelete: permanently removes a record, bypassing soft-delete.
//
// These operations are delegated to the inner CRUD if it implements the
// SoftDeleter[ID] interface. If it does not, ErrNotSoftDeletable is returned.
//
// Usage:
//
//	inner := r3pgx.NewPgxCRUD[User, int64](db)
//	repo := softdelete.WithSoftDelete[User, int64](inner)
//
//	// repo satisfies r3.CRUD[User, int64] — all methods pass through.
//	// Additionally:
//	repo.Restore(ctx, userID)    // un-deletes
//	repo.HardDelete(ctx, userID) // permanent delete
package softdelete
