package depo

import (
	"context"
)

// Repoable is a generic interface for CRUD operations with parametrized List.
type Repoable[T any, ID comparable] interface {
	// Create inserts a new record into the database.
	Create(ctx context.Context, entity T) (T, error)

	// Get retrieves a record by its ID with optional parameters.
	Get(ctx context.Context, id ID, params GetParams) (T, error)

	// List retrieves records based on the provided ListParams.
	List(ctx context.Context, params ListParams) ([]T, error)

	// Update modifies an existing record in the database with optional parameters.
	Update(ctx context.Context, entity T) (T, error)

	// Patch is a partial update (only given fields are being updated)
	Patch(ctx context.Context, entity T, fields Fieldables) (T, error)

	// PatchRaw is a partial update that accepts list of Patches instead of the model
	PatchRaw(ctx context.Context, id ID, patches ...Patchable) (T, error)

	// Delete removes a record by its ID.
	// It can use soft delete (if it's turned on the repository level)
	Delete(ctx context.Context, id ID) error
}
