// Package permissions provides a policy-based, entity-aware permission decorator
// for r3 CRUD repositories.
//
// It works as a pre-call gating decorator around any r3.CRUD[T, ID] implementation,
// checking authorization before every Create, Get, List, Update, Patch, and Delete
// operation. The decorator is stateless -- it doesn't store roles or permissions.
// A single Checker interface receives the actor, operation, and entity context,
// returning allow or deny. Users bring their own authorization logic.
//
// Key features:
//   - Policy-based: no built-in concept of roles or permission sets
//   - Entity-aware: the Checker receives the actual entity for row-level rules
//   - Scope injection: optional Scoper interface injects filters into List queries
//   - Composable helpers: AllowAll, DenyAll, ReadOnly, ByActorType, Compose, OperationCheckers
//   - r3.Actor integration: reads actor from context via r3.GetActor(ctx)
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
// When WithIDFunc is configured, the decorator fetches the existing entity before
// Update, Patch, and Delete operations, enabling row-level rules like "users can
// only edit their own posts":
//
//	repo := permissions.WithPermissions[Post, int64](
//	    innerRepo, myChecker,
//	    permissions.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
//	)
//
// # Scope Injection
//
// When a Checker also implements Scoper, the decorator injects filters into List
// queries for efficient DB-level row filtering:
//
//	type postPolicy struct{}
//	func (p postPolicy) Check(ctx context.Context, req permissions.AccessRequest[Post, int64]) error { ... }
//	func (p postPolicy) Scope(ctx context.Context, actor r3.Actor) (r3.Filters, error) {
//	    if actor.Type == "admin" { return nil, nil }
//	    return r3.Filters{r3.F(r3.NewFieldSpec("owner_id"), actor.ID)}, nil
//	}
package permissions
