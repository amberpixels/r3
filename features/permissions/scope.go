package permissions

import (
	"context"

	"github.com/amberpixels/r3"
)

// Scoper optionally narrows List queries based on the actor's permissions.
// When a Checker also implements Scoper, the decorator calls Scope() before
// List and merges the returned filters into the query.
//
// This enables efficient row-level access control at the database level
// (e.g., "WHERE owner_id = $actorID") instead of post-filtering in memory.
//
// Scoper is a separate optional interface, not part of Checker. This follows
// R3's pattern (like SoftDeleter being separate from CRUD). If a Checker
// doesn't implement Scoper, List just checks OpRead permission without
// filter injection.
type Scoper[T any, ID comparable] interface {
	Scope(ctx context.Context, actor r3.Actor) (r3.Filters, error)
}
