package permissions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/permissions"
)

// capMemory is a memoryCRUD that also implements the Upserter and BulkPatcher
// capabilities, recording what PatchWhere/Upsert received so tests can assert
// permission gating and scope injection.
type capMemory struct {
	*memoryCRUD
	upsertCalls    int
	lastPWFilters  r3.Filters
	patchWhereCall int
}

func newCapMemory() *capMemory { return &capMemory{memoryCRUD: newMemoryCRUD()} }

// denyOp builds a Checker that denies exactly one operation and allows the rest.
func denyOp(op permissions.Operation) permissions.Checker[Post, int64] {
	return permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			if req.Operation == op {
				return permissions.ErrAccessDenied
			}
			return nil
		})
}

func (m *capMemory) Upsert(_ context.Context, entity Post, _ ...r3.UpsertOption) (Post, error) {
	m.upsertCalls++
	if entity.ID == 0 {
		entity.ID = m.nextID
		m.nextID++
	}
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *capMemory) PatchWhere(
	_ context.Context, filters r3.Filters, _ Post, _ r3.Fields,
) (int64, error) {
	m.patchWhereCall++
	m.lastPWFilters = filters
	return int64(len(m.data)), nil
}

// postScope confines an actor to rows it owns (owner_id == actor.ID).
type postScope struct{}

func (postScope) Check(context.Context, permissions.AccessRequest[Post, int64]) error { return nil }

func (postScope) Scope(_ context.Context, actor r3.Actor) (r3.Filters, error) {
	return r3.Filters{r3.Eq("owner_id", actor.ID)}, nil
}

func TestPermissions_Upsert_RequiresCreateAndUpdate(t *testing.T) {
	ctx := context.Background()

	for name, checker := range map[string]permissions.Checker[Post, int64]{
		"deny-create": denyOp(permissions.OpCreate),
		"deny-update": denyOp(permissions.OpUpdate),
	} {
		t.Run(name, func(t *testing.T) {
			inner := newCapMemory()
			repo := permissions.WithPermissions[Post, int64](inner, checker)
			_, err := repo.Upsert(ctx, Post{Title: "x"})
			if err == nil {
				t.Fatalf("expected denial, got nil")
			}
			if inner.upsertCalls != 0 {
				t.Fatalf("inner Upsert must not run when denied")
			}
		})
	}

	// AllowAll passes both checks and reaches the backend.
	inner := newCapMemory()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())
	if _, err := repo.Upsert(ctx, Post{Title: "ok"}); err != nil {
		t.Fatalf("AllowAll upsert: %v", err)
	}
	if inner.upsertCalls != 1 {
		t.Fatalf("inner Upsert should have run once, got %d", inner.upsertCalls)
	}
}

func TestPermissions_Upsert_NotSupportedPassesThrough(t *testing.T) {
	// The plain memoryCRUD has no Upserter capability → the sentinel surfaces.
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())
	if _, err := repo.Upsert(context.Background(), Post{Title: "x"}); !errors.Is(err, r3.ErrUpsertNotSupported) {
		t.Fatalf("want ErrUpsertNotSupported, got %v", err)
	}
}

func TestPermissions_PatchWhere_ANDsInScopeFilters(t *testing.T) {
	ctx := r3.WithActor(context.Background(), r3.Actor{Type: "user", ID: "alice"})
	inner := newCapMemory()
	repo := permissions.WithPermissions[Post, int64](inner, postScope{})

	callerFilters := r3.Filters{r3.Eq("title", "draft")}
	if _, err := repo.PatchWhere(ctx, callerFilters, Post{Title: "published"},
		r3.Fields{r3.NewFieldSpec("title")}); err != nil {
		t.Fatalf("PatchWhere: %v", err)
	}

	// The backend must have seen caller filter AND the actor's scope filter.
	if len(inner.lastPWFilters) != 2 {
		t.Fatalf("expected caller+scope filters (2), got %d: %#v", len(inner.lastPWFilters), inner.lastPWFilters)
	}
	// The caller's slice must not be mutated.
	if len(callerFilters) != 1 {
		t.Fatalf("caller filter slice was mutated: len=%d", len(callerFilters))
	}
	// Confirm the scope filter (owner_id=alice) was the one appended.
	foundOwner := false
	for _, f := range inner.lastPWFilters {
		if f != nil && f.Field != nil && f.Field.String() == "owner_id" {
			foundOwner = true
		}
	}
	if !foundOwner {
		t.Fatalf("scope filter owner_id not AND-ed in: %#v", inner.lastPWFilters)
	}
}

func TestPermissions_PatchWhere_DeniedDoesNotReachBackend(t *testing.T) {
	inner := newCapMemory()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.DenyAll[Post, int64]())
	_, err := repo.PatchWhere(context.Background(), r3.Filters{r3.Eq("title", "x")},
		Post{}, r3.Fields{r3.NewFieldSpec("title")})
	if err == nil {
		t.Fatalf("expected denial")
	}
	if inner.patchWhereCall != 0 {
		t.Fatalf("backend PatchWhere must not run when denied")
	}
}
