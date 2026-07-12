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
	// OpUpsert validates the full entity of an Upsert (insert-or-update); the
	// whole entity is present, as on Create.
	OpUpsert Operation = "upsert"
	// OpPatchWhere validates the entity/fields of a bulk conditional update. Like
	// Patch, only Fields carry meaning; there is no single Existing row, so
	// Existing/Merged are never populated.
	OpPatchWhere Operation = "patch_where"
)

// Request carries the context for a validation decision.
type Request[T any, ID comparable] struct {
	// Operation is the mutation type.
	Operation Operation

	// Entity being validated. Always populated.
	Entity T

	// Fields being patched. Non-nil only for Patch; lets validators skip rules for
	// unchanged fields.
	Fields r3.Fields

	// Existing is the current DB state. Populated only for Update/Patch when
	// WithIDFunc is set - enables state-transition rules (e.g. "draft -> published").
	Existing *T

	// Merged is the entity AFTER the patch is applied (patched fields overlaid on
	// Existing). Populated only for Patch when WithIDFunc is set. Prefer Merged over
	// Entity for whole-entity rules on Patch, since Entity carries only the patched
	// fields (rest zeroed) and validating it directly would reject valid patches.
	Merged *T
}

// Validator validates entities before mutations, using any library or plain Go.
// Validate returns nil to allow or an error to reject; an *Error carries
// structured per-field details, but any error type prevents the mutation.
type Validator[T any, ID comparable] interface {
	Validate(ctx context.Context, req Request[T, ID]) error
}

// ValidatorFunc is an adapter to allow use of ordinary functions as Validators.
type ValidatorFunc[T any, ID comparable] func(ctx context.Context, req Request[T, ID]) error

// Validate calls the underlying function.
func (f ValidatorFunc[T, ID]) Validate(ctx context.Context, req Request[T, ID]) error {
	return f(ctx, req)
}
