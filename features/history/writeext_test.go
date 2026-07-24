package history_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/history"
)

// capMemory adds the Upserter/BulkPatcher capabilities to the in-memory Order
// CRUD, with a filter-aware List/PatchWhere so the history decorator's
// snapshot-then-diff logic can be exercised faithfully.
type capMemory struct {
	*memoryCRUD
}

func newCapMemory() *capMemory { return &capMemory{memoryCRUD: newMemoryCRUD()} }

func (m *capMemory) Upsert(_ context.Context, e Order, _ ...r3.UpsertOption) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e.ID == 0 {
		e.ID = m.nextID
		m.nextID++
	}
	m.data[e.ID] = e
	return e, nil
}

func (m *capMemory) List(_ context.Context, qarg ...r3.Query) ([]Order, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Order
	for _, v := range m.data {
		if len(qarg) > 0 && !orderMatches(v, qarg[0].Filters) {
			continue
		}
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *capMemory) PatchWhere(
	_ context.Context, filters r3.Filters, e Order, fields r3.Fields,
) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var n int64
	for id, cur := range m.data {
		if !orderMatches(cur, filters) {
			continue
		}
		for _, f := range fields {
			if f.String() == "status" {
				cur.Status = e.Status
			}
		}
		m.data[id] = cur
		n++
	}
	return n, nil
}

// orderMatches applies the status equality filter used by these tests.
func orderMatches(o Order, filters r3.Filters) bool {
	for _, f := range filters {
		if f == nil || f.Field == nil {
			continue
		}
		if f.Field.String() == "status" {
			if s, ok := f.Value.(string); ok && o.Status != s {
				return false
			}
		}
	}
	return true
}

func TestHistory_Upsert_RecordsCreateThenUpdate(t *testing.T) {
	crud := newCapMemory()
	store := newMemoryChangeRecordCRUD()
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)
	ctx := context.Background()

	// First upsert: no pre-state → recorded as a create.
	created, err := repo.Upsert(ctx, Order{Name: "o", Total: 100, Status: "pending"})
	be.NoError(t, err)

	recs, _, err := listForRecord(ctx, store, "orders", strconv.FormatInt(created.ID, 10))
	be.NoError(t, err)

	be.RequireThat(t, recs, be.HaveLength(1))
	be.RequireThat(t, recs[0].Action, be.Eq(history.ActionCreate))

	// Second upsert on the same key: pre-state exists → recorded as an update.
	created.Status = "confirmed"
	_, err = repo.Upsert(ctx, created)
	be.NoError(t, err)

	recs, _, err = listForRecord(ctx, store, "orders", strconv.FormatInt(created.ID, 10))
	be.NoError(t, err)

	be.RequireThat(t, recs, be.HaveLength(2))
	var sawUpdate bool
	for _, r := range recs {
		if r.Action == history.ActionUpdate {
			sawUpdate = true
			be.AssertThat(t, r.Changes.Val, be.NotEmpty())
		}
	}
	be.RequireThat(t, sawUpdate, be.True())
}

func TestHistory_PatchWhere_RecordsPerAffectedRow(t *testing.T) {
	crud := newCapMemory()
	store := newMemoryChangeRecordCRUD()
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)
	ctx := context.Background()

	a, _ := crud.Create(ctx, Order{Name: "a", Status: "running"})
	b, _ := crud.Create(ctx, Order{Name: "b", Status: "running"})
	_, _ = crud.Create(ctx, Order{Name: "c", Status: "done"})

	n, err := repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("status", "running")},
		Order{Status: "interrupted"},
		r3.Fields{r3.NewFieldSpec("status")},
	)
	be.NoError(t, err)

	be.RequireThat(t, n, be.Eq(int64(2)))

	// Exactly the two matching rows get an ActionPatch record; the "done" row does not.
	for _, id := range []int64{a.ID, b.ID} {
		recs, _, err := listForRecord(ctx, store, "orders", strconv.FormatInt(id, 10))
		be.NoError(t, err)

		be.RequireThat(t, recs, be.HaveLength(1))
		be.RequireThat(t, recs[0].Action, be.Eq(history.ActionPatch))
	}
}
