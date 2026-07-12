package permissions

import (
	"context"

	"github.com/amberpixels/r3"
)

// Scoper is an optional interface a Checker may also implement to narrow reads
// by actor. When present, the decorator merges Scope's filters into List/Count/
// Aggregate for row-level access control at the DB (e.g. "WHERE owner_id = ?")
// instead of post-filtering in memory; a Checker without it gets no injection.
//
// Plain column scope filters are enforced on single-entity Get in memory. A
// relationship ("has") scope filter (r3.Has) can't be matched in memory, so the
// decorator verifies Get through a backend query - which requires WithIDFunc.
// Without it, a relationship-scoped Get fails closed (r3.ErrNotFound) rather than
// risk leaking an out-of-scope row.
type Scoper[T any, ID comparable] interface {
	Scope(ctx context.Context, actor r3.Actor) (r3.Filters, error)
}
