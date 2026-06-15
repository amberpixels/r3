package validation

import (
	"context"

	"github.com/amberpixels/r3"
)

// Operation represents a mutation type that triggers validation.
// Read operations (Get, List) and Delete do not trigger validation.
type Operation string

const (
	OpCreate Operation = "create"
	OpUpdate Operation = "update"
	OpPatch  Operation = "patch"
)

// Request carries all context needed for a validation decision.
//
// Fields:
//   - Operation: which mutation is being performed
//   - Entity: the entity being validated (always populated)
//   - Fields: for Patch operations, which fields are being changed (nil for Create/Update)
//   - Existing: for Update/Patch, the current DB state (nil unless IDFunc is configured)
type Request[T any, ID comparable] struct {
	// Operation is the mutation type: create, update, or patch.
	Operation Operation

	// Entity is the entity being validated. Always populated.
	Entity T

	// Fields is the list of fields being patched. Non-nil only for Patch operations.
	// Validators can use this to skip rules for fields that aren't being changed.
	Fields r3.Fields

	// Existing is the current database state of the entity.
	// Populated only for Update and Patch operations when WithIDFunc is configured.
	// Enables state-transition validation (e.g. "status can only go draft -> published").
	Existing *T

	// Merged is the full entity as it will look AFTER the patch is applied: the
	// patched fields overlaid on Existing. Populated only for Patch when WithIDFunc
	// is configured. Validators should prefer Merged over Entity for whole-entity
	// rules on Patch, since Entity carries only the patched fields (the rest are
	// zeroed) — validating Entity directly would reject otherwise-valid patches.
	Merged *T
}

// Validator validates entities before mutation operations.
// Implementations can use any validation library (go-playground/validator,
// ozzo-validation, custom logic, etc.)
//
// Validate returns nil to allow the operation, or an error to reject it.
// Returning an *Error provides structured per-field error details.
// Any other error type is also accepted and will prevent the mutation.
type Validator[T any, ID comparable] interface {
	Validate(ctx context.Context, req Request[T, ID]) error
}

// ValidatorFunc is an adapter to allow use of ordinary functions as Validators.
type ValidatorFunc[T any, ID comparable] func(ctx context.Context, req Request[T, ID]) error

// Validate calls the underlying function.
func (f ValidatorFunc[T, ID]) Validate(ctx context.Context, req Request[T, ID]) error {
	return f(ctx, req)
}
