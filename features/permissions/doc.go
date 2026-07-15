// Package permissions gates every CRUD operation through a caller-supplied
// authorization policy. The decorator wraps any r3.CRUD[T, ID], transparently
// satisfies it, and drops in anywhere a CRUD is expected.
//
// It is stateless: it stores no roles or permissions. A single Checker receives
// the actor (read from context via r3.GetActor), operation, and entity context
// and returns allow or deny - bring your own authorization logic. Composable
// helpers cover the common shapes: AllowAll, DenyAll, ReadOnly, ByActorType,
// Compose, OperationCheckers.
//
// # Basic Usage
//
//	repo := permissions.WithPermissions[Post, int64](
//	    innerRepo,
//	    permissions.ByActorType[Post, int64](map[string]permissions.Checker[Post, int64]{
//	        "admin": permissions.AllowAll[Post, int64](),
//	        "user":  permissions.ReadOnly[Post, int64](),
//	    }),
//	)
//
// # Entity-Aware Checks
//
// With WithIDFunc, the decorator fetches the existing entity before Update,
// Patch, and Delete, enabling row-level rules like "users can only edit their
// own posts":
//
//	repo := permissions.WithPermissions[Post, int64](
//	    innerRepo, myChecker,
//	    permissions.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
//	)
//
// # Scope Injection
//
// When a Checker also implements Scoper, the decorator injects its filters into
// List/Count/Aggregate for DB-level row filtering (and enforces them on Get):
//
//	type postPolicy struct{}
//	func (p postPolicy) Check(ctx context.Context, req permissions.AccessRequest[Post, int64]) error { ... }
//	func (p postPolicy) Scope(ctx context.Context, actor r3.Actor) (r3.Filters, error) {
//	    if actor.Type == "admin" { return nil, nil }
//	    return r3.Filters{r3.F(r3.NewFieldSpec("owner_id"), actor.ID)}, nil
//	}
//
// # Advertisement
//
// Beyond enforcing, the package can advertise verdicts: Allow, AllowedOps, and
// AllowResource ask the same Checker/Scoper the decorator asks, without
// performing the operation. Use them to publish per-row capabilities (a DTO
// "can" block a frontend renders as flags) instead of re-implementing the
// policy client-side:
//
//	ops := permissions.AllowedOps[Post, int64](ctx, policy, post,
//	    permissions.OpUpdate, permissions.OpDelete)
//
// The helpers work on a bare Checker (no decorator needed - e.g. on rows from
// an unwrapped repo); the decorator's Allow/AllowedOps methods additionally
// populate AccessRequest.EntityID via WithIDFunc. See Allow for the exact
// faithfulness contract.
package permissions
