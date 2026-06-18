package r3_test

import (
	"context"
	"testing"

	"github.com/amberpixels/r3"
)

func TestWithActor_RoundTripsIDAndType(t *testing.T) {
	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "42", Type: "user"})

	got := r3.GetActor(ctx)
	if got.ID != "42" || got.Type != "user" {
		t.Fatalf("got %+v, want ID=42 Type=user", got)
	}
}

func TestGetActor_DefaultsToSystemActor(t *testing.T) {
	got := r3.GetActor(context.Background())
	if got != r3.SystemActor {
		t.Fatalf("got %+v, want SystemActor %+v", got, r3.SystemActor)
	}
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

	got := r3.GetActor(ctx)
	if got.ID != "42" || got.Type != "user" {
		t.Fatalf("identity not preserved: %+v", got)
	}

	p, ok := got.Claims.(*principal)
	if !ok {
		t.Fatalf("claims type assertion failed: %T", got.Claims)
	}
	if p != want {
		t.Fatalf("claims pointer changed: got %p want %p", p, want)
	}
	if p.Role != "squad-editor" || len(p.Squads) != 2 {
		t.Fatalf("claims value corrupted: %+v", p)
	}
}

func TestSystemActor_HasNilClaims(t *testing.T) {
	if r3.SystemActor.Claims != nil {
		t.Fatalf("SystemActor.Claims = %v, want nil", r3.SystemActor.Claims)
	}
}
