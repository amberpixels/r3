package permissions_test

import (
	"context"
	"testing"

	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
	"github.com/amberpixels/r3/features/permissions"
	"github.com/expectto/be"
)

// Sale is the aggregate-scoping fixture: tenant-keyed rows with an amount.
type Sale struct {
	ID       int    `json:"id"        r3:"id,pk"`
	TenantID int64  `json:"tenant_id"`
	Region   string `json:"region"`
	Amount   int    `json:"amount"`
}

// saleScope scopes by the tenant id carried in the actor claims; no claim
// (admin) sees everything. deniedScope refuses reads outright.
type saleScope struct{}

func (saleScope) Check(_ context.Context, _ permissions.AccessRequest[Sale, int]) error {
	return nil
}

func (saleScope) Scope(_ context.Context, actor r3.Actor) (r3.Filters, error) {
	tenant, ok := actor.Claims.(int64)
	if !ok {
		return nil, nil
	}
	return r3.Filters{r3.Eq("tenant_id", tenant)}, nil
}

type denyReads struct{}

func (denyReads) Check(_ context.Context, req permissions.AccessRequest[Sale, int]) error {
	if req.Operation == permissions.OpRead {
		return permissions.NewAccessDeniedError(req.Operation, req.Actor, "reads denied")
	}
	return nil
}

func newSaleRepo(t *testing.T) *enginefile.BaseCRUD[Sale, int] {
	t.Helper()
	repo, err := enginefile.New[Sale, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(t.TempDir()),
	)
	be.NoError(t, err, "new file repo")

	ctx := context.Background()
	for _, s := range []Sale{
		{TenantID: 1, Region: "north", Amount: 10},
		{TenantID: 1, Region: "north", Amount: 20},
		{TenantID: 1, Region: "south", Amount: 5},
		{TenantID: 2, Region: "north", Amount: 100},
		{TenantID: 2, Region: "south", Amount: 200},
	} {
		_, err := repo.Create(ctx, s)
		be.NoError(t, err, "seed")
	}
	return repo
}

// TestAggregate_ScopeFiltersApply is the load-bearing permissions case: a
// scoped actor's aggregate must only fold rows inside their scope — otherwise
// aggregation would be a side door around row visibility.
func TestAggregate_ScopeFiltersApply(t *testing.T) {
	repo := permissions.WithPermissions[Sale, int](newSaleRepo(t), saleScope{})

	q := r3.Query{
		GroupBy:    r3.GroupBy("region"),
		Aggregates: r3.Aggregates{r3.AggCount("n"), r3.AggSum("amount", "total")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("region"))},
	}

	// Tenant 1 sees only its 3 rows: north (10+20), south (5).
	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "u1", Type: "user", Claims: int64(1)})
	rows, err := r3.AggregateOf(ctx, repo, q)
	be.NoError(t, err, "scoped aggregate")
	be.RequireThat(t, rows, be.HaveLength(2), "expected 2 regions")
	total, _ := rows[0].Int64("total")
	be.AssertThat(t, total, be.Eq(int64(30)),
		"tenant 1 north total should be 30 (tenant 2's 100 must be invisible)")
	total, _ = rows[1].Int64("total")
	be.AssertThat(t, total, be.Eq(int64(5)),
		"tenant 1 south total should be 5 (tenant 2's 200 must be invisible)")

	// An unscoped admin aggregates everything.
	admin := r3.WithActor(context.Background(), r3.Actor{ID: "root", Type: "admin"})
	rows, err = r3.AggregateOf(admin, repo, q)
	be.NoError(t, err, "admin aggregate")
	total, _ = rows[0].Int64("total")
	be.AssertThat(t, total, be.Eq(int64(130)), "admin north total should be 130")
}

func TestAggregate_ReadDenied(t *testing.T) {
	repo := permissions.WithPermissions[Sale, int](
		newSaleRepo(t),
		permissions.CheckerFunc[Sale, int](denyReads{}.Check),
	)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "u1", Type: "user"})
	_, err := r3.AggregateOf(ctx, repo, r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	be.ErrorIs(t, err, permissions.ErrAccessDenied)
}

// TestAggregate_InnerWithoutSupport verifies the decorator surfaces the typed
// sentinel when the wrapped repo has no aggregation capability.
func TestAggregate_InnerWithoutSupport(t *testing.T) {
	inner := &docMem{data: map[int64]Doc{}}
	repo := permissions.WithPermissions[Doc, int64](inner, tenantScope{})

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "u", Type: "user"})
	_, err := r3.AggregateOf(ctx, repo, r3.Query{Aggregates: r3.Aggregates{r3.AggCount("n")}})
	be.ErrorIs(t, err, r3.ErrAggregateNotSupported)
}
