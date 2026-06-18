package r3

import "context"

// Actor represents the identity performing a CRUD operation.
// It is stored in context.Context and automatically picked up by features
// like metrics and history for attribution.
//
// A zero-value Actor represents the system/anonymous actor.
type Actor struct {
	// ID identifies who performed the action (e.g. user ID, API key ID, service name).
	ID string

	// Type categorizes the actor (e.g. "user", "service", "system", "cron").
	Type string

	// Claims carries optional application-defined authorization data attached to
	// the actor (roles, group/tenant memberships, scopes, capabilities, ...).
	// R3 itself never inspects Claims — it only propagates it on the context so
	// policies can read it back. The permissions feature passes it through to
	// Checker/Scoper via AccessRequest.Actor, letting an authorization policy ride
	// entirely on the canonical actor instead of a separate, parallel context key.
	//
	// It is intentionally untyped (any): the application owns the shape and
	// type-asserts it (commonly to a pointer of its own principal type). A nil
	// Claims is the norm for system/service actors and the attribution-only
	// features (metrics, history), which ignore it.
	Claims any
}

// SystemActor is the default actor used when no actor is set in the context.
var SystemActor = Actor{ID: "", Type: "system"}

type actorContextKey struct{}

// WithActor returns a new context with the given Actor attached.
// Typically called in HTTP middleware after authentication:
//
//	ctx := r3.WithActor(r.Context(), r3.Actor{ID: "42", Type: "user"})
//
// Authorization data can ride along on Claims, where policies read it back
// (e.g. via the permissions feature's AccessRequest.Actor):
//
//	ctx := r3.WithActor(r.Context(), r3.Actor{ID: "42", Type: "user", Claims: principal})
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// GetActor retrieves the Actor from the context.
// Returns SystemActor if no actor was set.
func GetActor(ctx context.Context) Actor {
	if a, ok := ctx.Value(actorContextKey{}).(Actor); ok {
		return a
	}
	return SystemActor
}
