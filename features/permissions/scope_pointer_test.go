package permissions_test

import (
	"context"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/permissions"
)

// Doc is an entity with a nullable foreign key (pointer field), the common shape
// for multi-tenant/squad scoping. It exercises that the Get-time scope matcher
// dereferences pointer fields before comparing against scalar filter values.
type Doc struct {
	ID       int64
	TenantID *int64
}

type docMem struct{ data map[int64]Doc }

func (m *docMem) Get(_ context.Context, id int64, _ ...r3.Query) (Doc, error) {
	d, ok := m.data[id]
	if !ok {
		return Doc{}, r3.ErrNotFound
	}
	return d, nil
}

func (m *docMem) List(_ context.Context, _ ...r3.Query) ([]Doc, int64, error) {
	out := make([]Doc, 0, len(m.data))
	for _, d := range m.data {
		out = append(out, d)
	}
	return out, int64(len(out)), nil
}

func (m *docMem) Count(_ context.Context, _ ...r3.Query) (int64, error) {
	return int64(len(m.data)), nil
}
func (m *docMem) Create(_ context.Context, d Doc) (Doc, error) { m.data[d.ID] = d; return d, nil }
func (m *docMem) Update(_ context.Context, d Doc) (Doc, error) { m.data[d.ID] = d; return d, nil }
func (m *docMem) Patch(_ context.Context, d Doc, _ r3.Fields) (Doc, error) {
	m.data[d.ID] = d
	return d, nil
}
func (m *docMem) Delete(_ context.Context, id int64) error { delete(m.data, id); return nil }

// tenantScope filters to a single tenant id carried on the actor's claims; nil
// claims (admin) sees everything.
type tenantScope struct{}

func (tenantScope) Check(_ context.Context, _ permissions.AccessRequest[Doc, int64]) error {
	return nil
}

func (tenantScope) Scope(_ context.Context, actor r3.Actor) (r3.Filters, error) {
	tenant, ok := actor.Claims.(int64)
	if !ok {
		return nil, nil // no claim => see everything
	}
	return r3.Filters{r3.In("tenant_id", []int64{tenant})}, nil
}

// TestScoper_GetMatchesPointerForeignKey is the regression for a *int64 (nullable
// FK) scoped column: the in-scope row must be visible and the out-of-scope row
// invisible (ErrNotFound). Before the fix the matcher compared the pointer
// itself, so even the in-scope row was wrongly hidden.
func TestScoper_GetMatchesPointerForeignKey(t *testing.T) {
	t1, t2 := int64(1), int64(2)
	inner := &docMem{data: map[int64]Doc{
		10: {ID: 10, TenantID: &t1},
		20: {ID: 20, TenantID: &t2},
		30: {ID: 30, TenantID: nil}, // unassigned
	}}
	repo := permissions.WithPermissions[Doc, int64](inner, tenantScope{})

	// Actor scoped to tenant 1.
	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "u", Type: "user", Claims: int64(1)})

	_, err := repo.Get(ctx, 10)
	be.NoError(t, err, "in-scope (tenant 1) row should be visible")
	_, err = repo.Get(ctx, 20)
	be.ErrorIs(t, err, r3.ErrNotFound, "out-of-scope (tenant 2) row should be ErrNotFound")
	_, err = repo.Get(ctx, 30)
	be.ErrorIs(t, err, r3.ErrNotFound, "unassigned (nil FK) row should be ErrNotFound for a scoped actor")

	// An unscoped actor (no claim) sees every row, including the nil-FK one.
	admin := r3.WithActor(context.Background(), r3.Actor{ID: "root", Type: "admin"})
	for _, id := range []int64{10, 20, 30} {
		_, err := repo.Get(admin, id)
		be.NoError(t, err, "admin should read every row")
	}
}
