package softdelete

import "context"

// SoftDeleter is an optional interface that CRUD implementations can satisfy
// to support soft-delete restore and hard-delete operations.
//
// Engine-based drivers (engine/sql, engine/mongo) and ORM drivers (gorm, bun, gopg)
// implement this interface. The decorator checks for it at runtime.
type SoftDeleter[ID comparable] interface {
	// Restore un-deletes a soft-deleted record by clearing its deleted_at field.
	Restore(ctx context.Context, id ID) error

	// HardDelete permanently removes a record, bypassing soft-delete.
	HardDelete(ctx context.Context, id ID) error
}
