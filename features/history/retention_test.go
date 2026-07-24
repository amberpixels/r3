package history_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3/features/history"
)

// TestRetention_CompactionPreservesReconstruction verifies M9: pruning old
// versions (here via MaxVersions) folds them into the oldest surviving record so
// diff-based Reconstruct still rebuilds correct state — it must not strip the v1
// baseline or intermediate diffs and corrupt revert.
func TestRetention_CompactionPreservesReconstruction(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)
	ctx := context.Background()

	// v1 create, then v2..v4 each touching a different field, so a naive prune of
	// the oldest versions would drop changes never repeated later.
	order, _ := repo.Create(ctx, Order{Name: "V1", Total: 100, Status: "a"})
	order.Name = "V2"
	order, _ = repo.Update(ctx, order)
	order.Total = 300
	order, _ = repo.Update(ctx, order)
	order.Status = "b"
	order, _ = repo.Update(ctx, order)
	// True latest state: Name=V2, Total=300, Status=b.

	recordID := strconv.FormatInt(order.ID, 10)
	reverter := repo.Reverter()

	// Sanity: full reconstruction before pruning.
	full, err := reverter.Reconstruct(ctx, recordID, 4)
	be.NoError(t, err)

	be.RequireThat(t, full.Name, be.Eq("V2"))
	be.RequireThat(t, full.Total, be.Eq(300))
	be.RequireThat(t, full.Status, be.Eq("b"))

	// Keep only the 2 most recent versions. v1+v2 must be folded into v3.
	enforcer := history.NewRetentionEnforcer(store, history.RetentionPolicy{MaxVersions: 2})
	deleted := enforcer.Enforce(ctx, "orders")
	be.RequireThat(t, deleted, be.Eq(int64(2)))

	remaining, _, err := store.List(ctx, history.QueryForRecord("orders", recordID))
	be.NoError(t, err)

	be.RequireThat(t, remaining, be.HaveLength(2))

	// Reconstruction of the latest version must still be correct — the folded
	// baseline on the oldest survivor (v3) carries the dropped fields.
	got, err := reverter.Reconstruct(ctx, recordID, 4)
	be.NoError(t, err)

	be.AssertThat(t, got.Name, be.Eq("V2"))
	be.AssertThat(t, got.Total, be.Eq(300))
	be.AssertThat(t, got.Status, be.Eq("b"))

	// The oldest surviving version (v3) must reconstruct to its own full state:
	// Name=V2 (from v2), Total=300 (v3), Status=a (still original at v3).
	v3, err := reverter.Reconstruct(ctx, recordID, 3)
	be.NoError(t, err)

	be.AssertThat(t, v3.Name, be.Eq("V2"))
	be.AssertThat(t, v3.Total, be.Eq(300))
	be.AssertThat(t, v3.Status, be.Eq("a"))
}
