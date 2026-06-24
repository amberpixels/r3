package r3_test

import (
	"context"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/expectto/be"
	"github.com/expectto/be/be_ctx"
	betestify "github.com/expectto/be/x/testify"
)

func TestWithActor_RoundTripsIDAndType(t *testing.T) {
	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "42", Type: "user"})

	// WithActor must return a real context.Context; be_ctx.Ctx() enforces that.
	betestify.Assert(t, ctx, be_ctx.Ctx())

	// The actor lives under an UNEXPORTED context key (actorContextKey{}), so the
	// key cannot be named from this external test package. We therefore can't use
	// be_ctx.CtxWithValue(theActorKey, ...) and must read it back via the public
	// getter and assert the result. (See friction note in the conversion report.)
	got := r3.GetActor(ctx)
	betestify.Assert(t, got.ID, be.Eq("42"))
	betestify.Assert(t, got.Type, be.Eq("user"))
}

func TestGetActor_DefaultsToSystemActor(t *testing.T) {
	got := r3.GetActor(context.Background())
	betestify.Assert(t, got, be.Eq(r3.SystemActor))
}

func TestActor_CarriesClaims(t *testing.T) {
	// Claims is application-defined; here a pointer to a custom principal type,
	// the common case (apps type-assert it back).
	type principal struct {
		Role   string
		Squads []int64
	}
	want := &principal{Role: "squad-editor", Squads: []int64{3, 5}}

	ctx := r3.WithActor(context.Background(), r3.Actor{
		ID:     "42",
		Type:   "user",
		Claims: want,
	})
	betestify.Assert(t, ctx, be_ctx.Ctx())

	got := r3.GetActor(ctx)
	betestify.Assert(t, got.ID, be.Eq("42"))
	betestify.Assert(t, got.Type, be.Eq("user"))

	p, ok := got.Claims.(*principal)
	betestify.Assert(t, ok, be.True())

	// Same pointer identity preserved through the round-trip.
	betestify.Assert(t, p, be.Eq(want))
	betestify.Assert(t, p.Role, be.Eq("squad-editor"))
	betestify.Assert(t, p.Squads, be.Eq([]int64{3, 5}))
}

func TestSystemActor_HasNilClaims(t *testing.T) {
	betestify.Assert(t, r3.SystemActor.Claims, be.Nil())
}
