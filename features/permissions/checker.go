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

// AccessRequest carries the context for a permission decision. Some fields are
// unset per operation: Entity is nil for List and pre-insert Create; EntityID is
// nil for List and Create.
type AccessRequest[T any, ID comparable] struct {
	Operation Operation // Which CRUD operation
	Actor     r3.Actor  // Who is performing it (from context)
	Entity    *T        // The entity being operated on (nil for List, Create pre-check)
	EntityID  *ID       // The entity ID (for Get, Update, Patch, Delete)
}

// Checker makes the authorization decision for a CRUD operation: return nil to
// allow, or an error (typically *AccessDeniedError) to deny. A non-nil error
// stops the operation before it reaches the inner CRUD. Implement it however you
// like - hardcoded rules, RBAC, ABAC, an external policy engine.
type Checker[T any, ID comparable] interface {
	Check(ctx context.Context, req AccessRequest[T, ID]) error
}

// CheckerFunc is an adapter to allow use of ordinary functions as Checkers.
type CheckerFunc[T any, ID comparable] func(ctx context.Context, req AccessRequest[T, ID]) error

// Check calls the underlying function.
func (f CheckerFunc[T, ID]) Check(ctx context.Context, req AccessRequest[T, ID]) error {
	return f(ctx, req)
}
