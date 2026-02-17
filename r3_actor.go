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
}

// SystemActor is the default actor used when no actor is set in the context.
var SystemActor = Actor{ID: "", Type: "system"}

type actorContextKey struct{}

// WithActor returns a new context with the given Actor attached.
// Typically called in HTTP middleware after authentication:
//
//	ctx := r3.WithActor(r.Context(), r3.Actor{ID: "42", Type: "user"})
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
