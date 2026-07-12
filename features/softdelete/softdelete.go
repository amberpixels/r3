package softdelete

import "context"

// SoftDeleter is the optional interface a CRUD satisfies to support restore and
// hard-delete. The engine drivers (engine/sql, engine/mongo) and ORM drivers
// (gorm, bun, gopg) implement it; the decorator checks for it at runtime.
type SoftDeleter[ID comparable] interface {
	// Restore un-deletes a soft-deleted record by clearing its deleted_at field.
	Restore(ctx context.Context, id ID) error

	// HardDelete permanently removes a record, bypassing soft-delete.
	HardDelete(ctx context.Context, id ID) error
}
