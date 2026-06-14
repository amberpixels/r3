package r3

import (
	"context"
)

// Querier is the read-only subset of repository operations.
// It provides methods for retrieving entities without modifying them.
//
// Use Querier when you need read-only access to a repository,
// for example when reading configuration or building reports.
type Querier[T any, ID comparable] interface {
	// Get retrieves a record by its ID with optional parameters.
	Get(context.Context, ID, ...Query) (T, error)

	// List retrieves records based on the provided query parameters.
	List(context.Context, ...Query) ([]T, int64, error)

	// Count returns the number of records matching the query's filters.
	//
	// Only Filters and IncludeTrashed affect the result — pagination, sorts,
	// fields, and preloads are ignored. Called with no query it counts every
	// (non-trashed) record. It is the efficient way to answer "how many?"
	// without materializing rows.
	Count(context.Context, ...Query) (int64, error)
}

// Commander is the write-only subset of repository operations.
// It provides methods for creating, modifying, and deleting entities.
type Commander[T any, ID comparable] interface {
	// Create inserts a new record into the database.
	Create(context.Context, T) (T, error)

	// Update modifies an existing record in the database with optional parameters.
	Update(context.Context, T) (T, error)

	// Patch performs a partial update, modifying only the columns specified by Fields.
	// The entity must have its primary key set. Only the fields named in the Fields
	// list are written to the database; all other columns remain unchanged.
	// Returns the full entity after the update.
	Patch(context.Context, T, Fields) (T, error)

	// Delete removes a record by its ID.
	// It can use soft delete (if it's turned on the repository level)
	Delete(context.Context, ID) error
}

// CRUD is the full read+write repository interface.
// It composes [Querier] (Get, List) and [Commander] (Create, Update, Patch, Delete).
type CRUD[T any, ID comparable] interface {
	Querier[T, ID]
	Commander[T, ID]
}
