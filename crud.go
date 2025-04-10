package r3

import (
	"context"

	r3atoms "github.com/amberpixels/r3/atoms"
)

// CRUD is main generic interface for CRUD repository operations.
type CRUD[T any, ID comparable] interface {
	// Create inserts a new record into the database.
	Create(context.Context, T) (T, error)

	// Get retrieves a record by its ID with optional parameters.
	Get(context.Context, ID, ...r3atoms.GetParams) (T, error)

	// List retrieves records based on the provided ListParams.
	List(context.Context, ...r3atoms.ListParams) ([]T, int64, error)

	// Update modifies an existing record in the database with optional parameters.
	Update(context.Context, T) (T, error)

	// Patch is a partial update (only given fields are being updated)
	Patch(context.Context, T, r3atoms.Fields) (T, error)

	// PatchRaw is a partial update that accepts list of Patches instead of the model
	PatchRaw(context.Context, ID, ...r3atoms.Patch) (T, error)

	// Delete removes a record by its ID.
	// It can use soft delete (if it's turned on the repository level)
	Delete(context.Context, ID) error
}
