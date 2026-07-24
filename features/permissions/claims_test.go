package permissions_test

import (
	"context"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/permissions"
)

// claimsPrincipal is an application-defined principal carried on r3.Actor.Claims.
// It models a tiny capability set: which owner IDs the actor may write.
type claimsPrincipal struct {
	writableOwners map[string]bool
}

// claimsChecker authorizes writes only for owners listed in the actor's claims,
// reading the principal entirely off req.Actor.Claims (no separate context key).
// Reads are always allowed.
func claimsChecker() permissions.Checker[Post, int64] {
	return permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			if req.Operation == permissions.OpRead {
				return nil
			}

			p, ok := req.Actor.Claims.(*claimsPrincipal)
			if !ok || p == nil {
				return permissions.NewAccessDeniedError(req.Operation, req.Actor, "no principal claims on actor")
			}
			if req.Entity != nil && p.writableOwners[req.Entity.OwnerID] {
				return nil
			}
			return permissions.NewAccessDeniedError(req.Operation, req.Actor, "owner not writable for this actor")
		},
	)
}

func TestPermissions_PolicyReadsClaimsFromActor(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](
		inner,
		claimsChecker(),
		permissions.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
	)

	// Seed a post owned by "alice".
	seedCtx := r3.WithActor(context.Background(), r3.Actor{
		ID: "1", Type: "user",
		Claims: &claimsPrincipal{writableOwners: map[string]bool{"alice": true}},
	})
	post, err := repo.Create(seedCtx, Post{Title: "hi", OwnerID: "alice"})
	be.NoError(t, err, "create as alice-writer should be allowed")

	// An actor whose claims do NOT include "alice" is denied the update,
	// purely from data carried on r3.Actor.Claims.
	bobCtx := r3.WithActor(context.Background(), r3.Actor{
		ID: "2", Type: "user",
		Claims: &claimsPrincipal{writableOwners: map[string]bool{"bob": true}},
	})
	post.Title = "edited"
	_, err = repo.Update(bobCtx, post)
	be.ErrorIs(t, err, permissions.ErrAccessDenied,
		"update by non-owner-writer should be denied")

	// The alice-writer is allowed.
	_, err = repo.Update(seedCtx, post)
	be.NoError(t, err, "update by alice-writer should be allowed")

	// Missing claims (system/anonymous actor) is denied for writes.
	_, err = repo.Update(context.Background(), post)
	be.ErrorIs(t, err, permissions.ErrAccessDenied, "update with no claims should be denied")
}
