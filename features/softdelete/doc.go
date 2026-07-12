// Package softdelete decorates any r3.CRUD[T, ID] with Restore and HardDelete,
// transparently satisfying the same interface and passing all standard methods
// through to the inner repo.
//
// Soft-delete filtering and Delete-as-UPDATE stay in the engines (engine/sql,
// engine/mongo) and ORM drivers (gorm, bun, gopg); this decorator only adds:
//
//   - Restore: un-deletes a soft-deleted record by clearing its deleted_at field.
//   - HardDelete: permanently removes a record, bypassing soft-delete.
//
// Both delegate to the inner CRUD when it implements SoftDeleter[ID], else return
// ErrNotSoftDeletable.
//
// Usage:
//
//	inner := r3pgx.NewPgxCRUD[User, int64](db)
//	repo := softdelete.WithSoftDelete[User, int64](inner)
//
//	// repo satisfies r3.CRUD[User, int64] - all methods pass through.
//	// Additionally:
//	repo.Restore(ctx, userID)    // un-deletes
//	repo.HardDelete(ctx, userID) // permanent delete
package softdelete
