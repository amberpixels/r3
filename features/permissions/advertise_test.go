package permissions_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/permissions"
	"github.com/expectto/be"
)

// ── Shared fixtures ──────────────────────────────────────────────────────

// ownerMutations allows reads for everyone and mutations only for the entity's
// owner - the row-level shape that motivated advertisement (the p44 "Lab"
// button case).
var ownerMutations = permissions.CheckerFunc[Post, int64](
	func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
		if req.Operation == permissions.OpRead {
			return nil
		}
		if req.Entity != nil && req.Entity.OwnerID == req.Actor.ID {
			return nil
		}
		return permissions.NewAccessDeniedError(req.Operation, req.Actor, "only the owner can modify this post")
	},
)

func actorCtx(id, typ string) context.Context {
	return r3.WithActor(context.Background(), r3.Actor{ID: id, Type: typ})
}

// ── Tests: Allow (unit, ownership row-level) ─────────────────────────────

func TestAllow_OwnershipRowLevel(t *testing.T) {
	post := Post{ID: 1, Title: "My Post", OwnerID: "user42"}

	ownerCtx := actorCtx("user42", "user")
	otherCtx := actorCtx("user99", "user")

	// The owner may update and delete their row.
	be.AssertThat(t, permissions.Allow[Post, int64](ownerCtx, ownerMutations, permissions.OpUpdate, post),
		be.True(), "owner update")
	be.AssertThat(t, permissions.Allow[Post, int64](ownerCtx, ownerMutations, permissions.OpDelete, post),
		be.True(), "owner delete")

	// A foreign actor may not - the exact verdict the frontend needs per row.
	be.AssertThat(t, permissions.Allow[Post, int64](otherCtx, ownerMutations, permissions.OpUpdate, post),
		be.False(), "non-owner update")
	be.AssertThat(t, permissions.Allow[Post, int64](otherCtx, ownerMutations, permissions.OpDelete, post),
		be.False(), "non-owner delete")

	// Reads are open (no Scoper on this checker).
	be.AssertThat(t, permissions.Allow[Post, int64](otherCtx, ownerMutations, permissions.OpRead, post),
		be.True(), "non-owner read")
}

// ── Tests: AllowedOps ────────────────────────────────────────────────────

func TestAllowedOps_DefaultSet(t *testing.T) {
	post := Post{ID: 1, Title: "My Post", OwnerID: "user42"}

	// Owner: full CRUD set, in canonical order.
	ops := permissions.AllowedOps[Post, int64](actorCtx("user42", "user"), ownerMutations, post)
	be.AssertThat(t, ops, be.Eq([]permissions.Operation{
		permissions.OpCreate, permissions.OpRead, permissions.OpUpdate, permissions.OpDelete,
	}), "owner default set")

	// Non-owner: read only.
	ops = permissions.AllowedOps[Post, int64](actorCtx("user99", "user"), ownerMutations, post)
	be.AssertThat(t, ops, be.Eq([]permissions.Operation{permissions.OpRead}), "non-owner default set")
}

func TestAllowedOps_PreservesGivenOrder(t *testing.T) {
	post := Post{ID: 1, Title: "My Post", OwnerID: "user42"}

	ops := permissions.AllowedOps[Post, int64](actorCtx("user42", "user"), ownerMutations, post,
		permissions.OpDelete, permissions.OpRead)
	be.AssertThat(t, ops, be.Eq([]permissions.Operation{permissions.OpDelete, permissions.OpRead}),
		"explicit ops keep their order")

	ops = permissions.AllowedOps[Post, int64](actorCtx("user99", "user"), ownerMutations, post,
		permissions.OpDelete, permissions.OpRead)
	be.AssertThat(t, ops, be.Eq([]permissions.Operation{permissions.OpRead}), "denied ops are dropped")
}

// ── Tests: AllowResource (row-less probe) ────────────────────────────────

func TestAllowResource(t *testing.T) {
	readOnly := permissions.ReadOnly[Post, int64]()
	ctx := actorCtx("user42", "user")

	be.AssertThat(t, permissions.AllowResource(ctx, readOnly, permissions.OpRead), be.True(), "read")
	be.AssertThat(t, permissions.AllowResource(ctx, readOnly, permissions.OpCreate), be.False(), "create")
	be.AssertThat(t, permissions.AllowResource(ctx, readOnly, permissions.OpUpdate), be.False(), "update")
	be.AssertThat(t, permissions.AllowResource(ctx, readOnly, permissions.OpDelete), be.False(), "delete")
}

func TestAllowResource_BuildsResourceLevelRequest(t *testing.T) {
	// AllowResource must issue the decorator's List-shaped request: Entity and
	// EntityID both nil - never a zero-value entity a policy could misjudge.
	var captured permissions.AccessRequest[Post, int64]
	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			captured = req
			return nil
		},
	)

	ok := permissions.AllowResource[Post, int64](actorCtx("user42", "user"), checker, permissions.OpCreate)
	be.AssertThat(t, ok, be.True())
	be.AssertThat(t, captured.Operation, be.Eq(permissions.OpCreate))
	be.AssertThat(t, captured.Actor.ID, be.Eq("user42"))
	be.AssertThat(t, captured.Entity, be.Nil(), "Entity must be nil for a resource-level probe")
	be.AssertThat(t, captured.EntityID, be.Nil(), "EntityID must be nil for a resource-level probe")
}

// ── Tests: Scoper folds into the read verdict ────────────────────────────

func TestAllow_ScoperFoldsIntoRead(t *testing.T) {
	alicePost := Post{ID: 1, Title: "Alice Post", OwnerID: "alice"}
	bobPost := Post{ID: 2, Title: "Bob Post", OwnerID: "bob"}

	// scopedPolicy (decorator_test.go): Check allows every read; the Scoper
	// narrows rows to owner_id == actor.ID; admins are unscoped.
	var policy permissions.Checker[Post, int64] = scopedPolicy{}
	aliceCtx := actorCtx("alice", "user")

	be.AssertThat(t, permissions.Allow(aliceCtx, policy, permissions.OpRead, alicePost),
		be.True(), "in-scope row is readable")
	be.AssertThat(t, permissions.Allow(aliceCtx, policy, permissions.OpRead, bobPost),
		be.False(), "out-of-scope row is not readable, even though Check(OpRead) passes")

	adminCtx := actorCtx("root", "admin")
	be.AssertThat(t, permissions.Allow(adminCtx, policy, permissions.OpRead, bobPost),
		be.True(), "unscoped admin reads everything")

	// Writes stay Check-only: scopedPolicy denies all mutations.
	be.AssertThat(t, permissions.Allow(aliceCtx, policy, permissions.OpUpdate, alicePost),
		be.False(), "scoper never authorises a write")
}

func TestAllow_RelationScopeFailsClosed(t *testing.T) {
	// relScoper (decorator_test.go) scopes reads via a relationship ("has")
	// filter, which pure in-memory advertisement cannot evaluate: fail closed,
	// exactly like the decorator's Get without WithIDFunc.
	var policy permissions.Checker[Post, int64] = relScoper{}
	ctx := actorCtx("u", "user")

	be.AssertThat(t, permissions.Allow(ctx, policy, permissions.OpRead, Post{ID: 1}),
		be.False(), "relation-scoped read must fail closed in advertisement")
}

type errScoper struct{}

func (errScoper) Check(_ context.Context, _ permissions.AccessRequest[Post, int64]) error {
	return nil
}

func (errScoper) Scope(_ context.Context, _ r3.Actor) (r3.Filters, error) {
	return nil, errors.New("scope backend unavailable")
}

func TestAllow_ScoperErrorFailsClosed(t *testing.T) {
	var policy permissions.Checker[Post, int64] = errScoper{}
	be.AssertThat(t, permissions.Allow(actorCtx("u", "user"), policy, permissions.OpRead, Post{ID: 1}),
		be.False(), "a Scoper error must fail closed, as the decorator fails the read")
}

// ── Tests: decorator methods + EntityID ──────────────────────────────────

// idReadingChecker keys its delete decision on req.EntityID - the request field
// package-level Allow can never populate (documented divergence): only the
// decorator's Allow, armed with IDFunc, advertises such a Checker faithfully.
var idReadingChecker = permissions.CheckerFunc[Post, int64](
	func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
		if req.Operation != permissions.OpDelete {
			return nil
		}
		if req.EntityID == nil {
			return permissions.NewAccessDeniedError(req.Operation, req.Actor, "no entity id")
		}
		return nil
	},
)

func TestDecoratorAllow_PopulatesEntityIDViaIDFunc(t *testing.T) {
	post := Post{ID: 7, Title: "Post", OwnerID: "user42"}
	inner := newMemoryCRUD()
	inner.data[7] = post
	inner.nextID = 8

	repo := permissions.WithPermissions[Post, int64](
		inner, idReadingChecker,
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)
	ctx := actorCtx("user42", "user")

	// The bare helper cannot supply EntityID: this checker denies. That is the
	// documented boundary of the package-level guarantee.
	be.AssertThat(t, permissions.Allow[Post, int64](ctx, idReadingChecker, permissions.OpDelete, post),
		be.False(), "package-level Allow has no EntityID")

	// The decorator method populates EntityID from IDFunc and must agree with
	// real enforcement.
	be.AssertThat(t, repo.Allow(ctx, permissions.OpDelete, post), be.True(), "decorator Allow with IDFunc")
	be.NoError(t, repo.Delete(ctx, 7), "enforcement agrees with the decorator's advertisement")
}

func TestDecoratorAllow_NoEntityIDForCreate(t *testing.T) {
	var captured permissions.AccessRequest[Post, int64]
	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			captured = req
			return nil
		},
	)

	repo := permissions.WithPermissions[Post, int64](
		newMemoryCRUD(), checker,
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	ok := repo.Allow(actorCtx("u", "user"), permissions.OpCreate, Post{Title: "New"})
	be.AssertThat(t, ok, be.True())
	be.AssertThat(t, captured.EntityID, be.Nil(), "Create never carries an EntityID, matching the decorator")
}

func TestDecoratorAllowedOps(t *testing.T) {
	post := Post{ID: 1, Title: "My Post", OwnerID: "user42"}
	repo := permissions.WithPermissions[Post, int64](
		newMemoryCRUD(), ownerMutations,
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	ops := repo.AllowedOps(actorCtx("user99", "user"), post, permissions.OpUpdate, permissions.OpDelete)
	be.AssertThat(t, ops, be.HaveLength(0), "non-owner gets no write caps")

	ops = repo.AllowedOps(actorCtx("user42", "user"), post, permissions.OpUpdate, permissions.OpDelete)
	be.AssertThat(t, ops, be.Eq([]permissions.Operation{permissions.OpUpdate, permissions.OpDelete}))
}

// ── Tests: parity (anti-drift) ───────────────────────────────────────────

// parityPolicy is a realistic combined policy: admins may do anything; reads
// pass Check (row visibility is the Scoper's job); mutations require ownership.
type parityPolicy struct{}

func (parityPolicy) Check(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
	if req.Actor.Type == "admin" {
		return nil
	}
	if req.Operation == permissions.OpRead {
		return nil
	}
	if req.Entity != nil && req.Entity.OwnerID == req.Actor.ID {
		return nil
	}
	return permissions.NewAccessDeniedError(req.Operation, req.Actor, "not the owner")
}

func (parityPolicy) Scope(_ context.Context, actor r3.Actor) (r3.Filters, error) {
	if actor.Type == "admin" {
		return nil, nil
	}
	return r3.Filters{r3.F(r3.NewFieldSpec("owner_id"), actor.ID)}, nil
}

// TestAdvertise_ParityWithEnforcement is the anti-drift contract: for a full
// (actor, op, entity) matrix, the advertised verdict - package-level Allow AND
// the decorator's Allow - must equal driving the real decorator, where "denied"
// is any error from the op (access denied for writes, ErrNotFound masking for
// out-of-scope reads). The inner repo allows everything, so the permission
// layer is the only possible failure. A future change to enforcement that
// forgets advertisement (or vice versa) fails here.
//
// Note the matrix uses WithIDFunc (the row-level setup advertisement targets)
// and a policy deciding from Entity fields, not EntityID - the documented scope
// of the package-level guarantee (see TestDecoratorAllow_PopulatesEntityIDViaIDFunc
// for the EntityID boundary).
func TestAdvertise_ParityWithEnforcement(t *testing.T) {
	alicePost := Post{ID: 1, Title: "Alice Post", OwnerID: "alice"}
	bobPost := Post{ID: 2, Title: "Bob Post", OwnerID: "bob"}

	actors := []r3.Actor{
		{ID: "alice", Type: "user"},
		{ID: "bob", Type: "user"},
		{ID: "root", Type: "admin"},
	}
	entities := []Post{alicePost, bobPost}
	ops := []permissions.Operation{
		permissions.OpCreate, permissions.OpRead, permissions.OpUpdate, permissions.OpDelete,
	}

	var policy permissions.Checker[Post, int64] = parityPolicy{}

	for _, actor := range actors {
		for _, entity := range entities {
			for _, op := range ops {
				t.Run(fmt.Sprintf("%s_%s_post%d", actor.ID, op, entity.ID), func(t *testing.T) {
					// Fresh state per case: Create and Delete mutate the store.
					inner := newMemoryCRUD()
					inner.data[1] = alicePost
					inner.data[2] = bobPost
					inner.nextID = 3
					repo := permissions.WithPermissions[Post, int64](
						inner, policy,
						permissions.WithIDFunc[Post, int64](postIDFunc),
					)
					ctx := r3.WithActor(context.Background(), actor)

					// Ground truth for this policy: admin or owner.
					expected := actor.Type == "admin" || entity.OwnerID == actor.ID

					// Advertisement, both entry points.
					be.AssertThat(t, permissions.Allow(ctx, policy, op, entity), be.Eq(expected),
						"package-level Allow")
					be.AssertThat(t, repo.Allow(ctx, op, entity), be.Eq(expected),
						"decorator Allow")

					// Enforcement: drive the real operation.
					var err error
					switch op {
					case permissions.OpCreate:
						_, err = repo.Create(ctx, entity)
					case permissions.OpRead:
						_, err = repo.Get(ctx, entity.ID)
					case permissions.OpUpdate:
						_, err = repo.Update(ctx, entity)
					case permissions.OpDelete:
						err = repo.Delete(ctx, entity.ID)
					default:
						t.Fatalf("unexpected op %q", op)
					}
					be.AssertThat(t, err == nil, be.Eq(expected), "enforcement verdict")
				})
			}
		}
	}
}
