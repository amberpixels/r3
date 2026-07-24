package r3_test

import (
	"context"
	"testing"

	"github.com/expectto/be"
	"github.com/expectto/be/be_ctx"

	"github.com/amberpixels/r3"
)

func TestWithActor_RoundTripsIDAndType(t *testing.T) {
	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "42", Type: "user"})

	// WithActor must return a real context.Context; be_ctx.Ctx() enforces that.
	be.AssertThat(t, ctx, be_ctx.Ctx())

	// The actor lives under an UNEXPORTED context key (actorContextKey{}), so the
	// key cannot be named from this external test package. We therefore can't use
	// be_ctx.CtxWithValue(theActorKey, ...) and must read it back via the public
	// getter and assert the result. (See friction note in the conversion report.)
	got := r3.GetActor(ctx)
	be.AssertThat(t, got.ID, be.Eq("42"))
	be.AssertThat(t, got.Type, be.Eq("user"))
}

func TestGetActor_DefaultsToSystemActor(t *testing.T) {
	got := r3.GetActor(context.Background())
	be.AssertThat(t, got, be.Eq(r3.SystemActor))
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
	be.AssertThat(t, ctx, be_ctx.Ctx())

	got := r3.GetActor(ctx)
	be.AssertThat(t, got.ID, be.Eq("42"))
	be.AssertThat(t, got.Type, be.Eq("user"))

	p, ok := got.Claims.(*principal)
	be.AssertThat(t, ok, be.True())

	// Same pointer identity preserved through the round-trip.
	be.AssertThat(t, p, be.Eq(want))
	be.AssertThat(t, p.Role, be.Eq("squad-editor"))
	be.AssertThat(t, p.Squads, be.Eq([]int64{3, 5}))
}

func TestSystemActor_HasNilClaims(t *testing.T) {
	be.AssertThat(t, r3.SystemActor.Claims, be.Nil())
}
