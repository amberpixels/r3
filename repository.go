package crood

import (
	"context"
)

// Repoable is a generic interface for CRUD operations with parametrized List.
type Repoable[T any, ID comparable] interface {
	// Create inserts a new record into the database.
	Create(ctx context.Context, entity T) (T, error)

	// Get retrieves a record by its ID with optional parameters.
	Get(ctx context.Context, id ID, params GetParams) (T, error)

	// Update modifies an existing record in the database with optional parameters.
	Update(ctx context.Context, entity T) (T, error)

	Patch(ctx context.Context, entity T) (T, error)
	PatchRaw(ctx context.Context, id ID, updates map[Fieldable]any) (T, error)

	// Delete removes a record by its ID.
	Delete(ctx context.Context, id ID) error

	// List retrieves records based on the provided ListParams.
	List(ctx context.Context, params ListParams) ([]T, error)
}
