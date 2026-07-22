package r3

import (
	"context"
)

// Querier is the read-only subset of repository operations. Depend on it when
// read-only access is all you need.
type Querier[T any, ID comparable] interface {
	// Get retrieves a record by its ID, normalizing not-found to [ErrNotFound].
	Get(context.Context, ID, ...Query) (T, error)

	// List retrieves records matching the query.
	List(context.Context, ...Query) ([]T, int64, error)

	// Count returns how many records match, without materializing rows. Only
	// Filters and IncludeTrashed apply; pagination/sorts/fields/preloads are
	// ignored, and no query counts every non-trashed record.
	Count(context.Context, ...Query) (int64, error)
}

// Commander is the write-only subset of repository operations.
type Commander[T any, ID comparable] interface {
	// Create inserts a new record.
	Create(context.Context, T) (T, error)

	// Update modifies an existing record and returns it as persisted. Columns the
	// backend does not write (immutable ones like created_at) come back with
	// their stored values, not the caller's; relation fields are returned as
	// passed in, since a read-back does not load them.
	Update(context.Context, T) (T, error)

	// Patch partially updates the row (PK must be set), writing only the columns
	// named in Fields and returning the full entity.
	Patch(context.Context, T, Fields) (T, error)

	// Delete removes a record by ID (soft-deletes if enabled on the repository).
	Delete(context.Context, ID) error
}

// CRUD is the full read+write repository interface, composing [Querier] and
// [Commander].
type CRUD[T any, ID comparable] interface {
	Querier[T, ID]
	Commander[T, ID]
}
