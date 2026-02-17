package permissions

import (
	"context"

	"github.com/amberpixels/r3"
)

// Operation represents a CRUD operation type.
type Operation string

const (
	OpCreate Operation = "create"
	OpRead   Operation = "read"   // Get and List
	OpUpdate Operation = "update" // Update and Patch
	OpDelete Operation = "delete"
)

// AccessRequest contains all context needed for a permission decision.
// Not all fields are populated for every operation:
//   - Entity is nil for List and pre-insert Create checks
//   - EntityID is nil for List and Create
type AccessRequest[T any, ID comparable] struct {
	Operation Operation // Which CRUD operation
	Actor     r3.Actor  // Who is performing it (from context)
	Entity    *T        // The entity being operated on (nil for List, Create pre-check)
	EntityID  *ID       // The entity ID (for Get, Update, Patch, Delete)
}

// Checker makes authorization decisions for CRUD operations.
// Implementations can be as simple or complex as needed:
// hardcoded rules, RBAC, ABAC, or external policy engines.
//
// Check returns nil to allow, or an error (typically *AccessDeniedError) to deny.
// Returning a non-nil error prevents the operation from reaching the inner CRUD.
type Checker[T any, ID comparable] interface {
	Check(ctx context.Context, req AccessRequest[T, ID]) error
}

// CheckerFunc is an adapter to allow use of ordinary functions as Checkers.
type CheckerFunc[T any, ID comparable] func(ctx context.Context, req AccessRequest[T, ID]) error

// Check calls the underlying function.
func (f CheckerFunc[T, ID]) Check(ctx context.Context, req AccessRequest[T, ID]) error {
	return f(ctx, req)
}
