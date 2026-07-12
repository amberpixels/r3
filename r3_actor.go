package r3

import "context"

// Actor is the identity performing a CRUD operation, carried in context and
// picked up by the metrics and history features for attribution. The zero value
// is the system/anonymous actor.
type Actor struct {
	// ID identifies who performed the action (e.g. user ID, API key ID, service name).
	ID string

	// Type categorizes the actor (e.g. "user", "service", "system", "cron").
	Type string

	// Claims carries optional application-defined authorization data (roles,
	// tenant memberships, scopes, ...). R3 never inspects it - it only rides the
	// context so policies can read it back; the permissions feature forwards it to
	// Checker/Scoper via AccessRequest.Actor. Intentionally untyped: the
	// application owns the shape and type-asserts it. Nil for system/service actors
	// and the attribution-only features, which ignore it.
	Claims any
}

// SystemActor is the actor used when the context carries none.
var SystemActor = Actor{ID: "", Type: "system"}

type actorContextKey struct{}

// WithActor attaches an Actor to the context, typically in auth middleware.
// Authorization data can ride along on [Actor]'s Claims:
//
//	ctx := r3.WithActor(r.Context(), r3.Actor{ID: "42", Type: "user", Claims: principal})
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// GetActor returns the Actor from the context, or [SystemActor] if none is set.
func GetActor(ctx context.Context) Actor {
	if a, ok := ctx.Value(actorContextKey{}).(Actor); ok {
		return a
	}
	return SystemActor
}
