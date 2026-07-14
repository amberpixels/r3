package history_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/history"
	"github.com/expectto/be"
	"github.com/expectto/be/be_string"
	"github.com/expectto/be/be_time"
)

// ── In-Memory CRUD (mock for entity) ─────────────────────────────────────

type Order struct {
	ID     int64  `db:"id,pk"  json:"id"`
	Name   string `db:"name"   json:"name"`
	Total  int    `db:"total"  json:"total"`
	Status string `db:"status" json:"status"`
}

type memoryCRUD struct {
	mu     sync.Mutex
	data   map[int64]Order
	nextID int64
}

func newMemoryCRUD() *memoryCRUD {
	return &memoryCRUD{data: make(map[int64]Order), nextID: 1}
}

func (m *memoryCRUD) Create(_ context.Context, entity Order) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity.ID = m.nextID
	m.nextID++
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Get(_ context.Context, id int64, _ ...r3.Query) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity, ok := m.data[id]
	if !ok {
		return Order{}, fmt.Errorf("not found: %d", id)
	}
	return entity, nil
}

func (m *memoryCRUD) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, qarg...)
	return n, err
}

func (m *memoryCRUD) List(_ context.Context, _ ...r3.Query) ([]Order, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Order
	for _, v := range m.data {
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *memoryCRUD) Update(_ context.Context, entity Order) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[entity.ID]; !ok {
		return Order{}, fmt.Errorf("not found: %d", entity.ID)
	}
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Patch(_ context.Context, entity Order, fields r3.Fields) (Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.data[entity.ID]
	if !ok {
		return Order{}, fmt.Errorf("not found: %d", entity.ID)
	}
	for _, f := range fields {
		switch f.String() {
		case "name":
			existing.Name = entity.Name
		case "total":
			existing.Total = entity.Total
		case "status":
			existing.Status = entity.Status
		}
	}
	m.data[entity.ID] = existing
	return existing, nil
}

func (m *memoryCRUD) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[id]; !ok {
		return fmt.Errorf("not found: %d", id)
	}
	delete(m.data, id)
	return nil
}

// ── In-Memory CRUD for ChangeRecord ──────────────────────────────────────
//
// This implements r3.CRUD[history.ChangeRecord, string] and replaces the
// old history.Store interface. The in-memory implementation interprets
// r3.Query filters to support the query builders from queries.go.

type memoryChangeRecordCRUD struct {
	mu      sync.Mutex
	records []history.ChangeRecord
}

func newMemoryChangeRecordCRUD() *memoryChangeRecordCRUD {
	return &memoryChangeRecordCRUD{}
}

func (s *memoryChangeRecordCRUD) Create(_ context.Context, record history.ChangeRecord) (history.ChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return record, nil
}

func (s *memoryChangeRecordCRUD) Get(_ context.Context, id string, _ ...r3.Query) (history.ChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.records {
		if r.ID == id {
			return r, nil
		}
	}
	return history.ChangeRecord{}, history.ErrRecordNotFound
}

func (s *memoryChangeRecordCRUD) Count(ctx context.Context, qargs ...r3.Query) (int64, error) {
	_, n, err := s.List(ctx, qargs...)
	return n, err
}

func (s *memoryChangeRecordCRUD) List(_ context.Context, qargs ...r3.Query) ([]history.ChangeRecord, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]history.ChangeRecord, len(s.records))
	copy(result, s.records)

	// Apply filters
	if len(qargs) > 0 {
		q := qargs[0]
		result = filterChangeRecords(result, q.Filters)

		// Apply sorts
		for _, ss := range q.Sorts {
			col := ss.Column.String()
			switch col {
			case "version":
				if ss.Direction == r3.SortDirectionAsc {
					sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
				} else {
					sort.Slice(result, func(i, j int) bool { return result[i].Version > result[j].Version })
				}
			case "created_at":
				if ss.Direction == r3.SortDirectionAsc {
					sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
				} else {
					sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
				}
			}
		}

		// Apply pagination (limit)
		if q.Pagination != nil && q.Pagination.PageSize.Some() {
			limit := q.Pagination.PageSize.Unwrap()
			if limit > 0 && limit < len(result) {
				result = result[:limit]
			}
		}
	}

	return result, int64(len(result)), nil
}

func (s *memoryChangeRecordCRUD) Update(_ context.Context, record history.ChangeRecord) (history.ChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.records {
		if r.ID == record.ID {
			s.records[i] = record
			return record, nil
		}
	}
	return record, history.ErrRecordNotFound
}

func (s *memoryChangeRecordCRUD) Patch(
	_ context.Context,
	record history.ChangeRecord,
	_ r3.Fields,
) (history.ChangeRecord, error) {
	return record, nil
}

func (s *memoryChangeRecordCRUD) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.records {
		if r.ID == id {
			s.records = append(s.records[:i], s.records[i+1:]...)
			return nil
		}
	}
	return history.ErrRecordNotFound
}

// filterChangeRecords applies r3 filters to a slice of ChangeRecords.
// Supports the filter patterns used by the query builders.
func filterChangeRecords(records []history.ChangeRecord, filters r3.Filters) []history.ChangeRecord {
	if len(filters) == 0 {
		return records
	}

	var result []history.ChangeRecord
	for _, rec := range records {
		if matchesFilters(rec, filters) {
			result = append(result, rec)
		}
	}
	return result
}

func matchesFilters(rec history.ChangeRecord, filters r3.Filters) bool {
	for _, f := range filters {
		if !matchesFilter(rec, f) {
			return false
		}
	}
	return true
}

func matchesFilter(rec history.ChangeRecord, f *r3.FilterSpec) bool {
	// AND group
	if len(f.And) > 0 {
		for _, child := range f.And {
			if !matchesFilter(rec, child) {
				return false
			}
		}
		return true
	}

	// OR group
	if len(f.Or) > 0 {
		for _, child := range f.Or {
			if matchesFilter(rec, child) {
				return true
			}
		}
		return false
	}

	// Leaf filter (Eq)
	if f.Field == nil {
		return true
	}
	fieldName := f.Field.String()
	val := f.Value

	switch fieldName {
	case "record_type":
		return rec.RecordType == fmt.Sprint(val)
	case "record_id":
		return rec.RecordID == fmt.Sprint(val)
	case "version":
		// Handle int64 comparison.
		switch v := val.(type) {
		case int64:
			return rec.Version == v
		case int:
			return rec.Version == int64(v)
		default:
			return strconv.FormatInt(rec.Version, 10) == fmt.Sprint(val)
		}
	case "parent_type":
		return rec.ParentType == fmt.Sprint(val)
	case "parent_id":
		return rec.ParentID == fmt.Sprint(val)
	default:
		return false
	}
}

// listForRecord queries records for a specific entity.
func listForRecord(
	ctx context.Context,
	store r3.CRUD[history.ChangeRecord, string],
	recordType, recordID string,
) ([]history.ChangeRecord, int64, error) {
	q := history.QueryForRecord(recordType, recordID)
	return store.List(ctx, q)
}

// ── In-Memory CRUD for Snapshot ──────────────────────────────────────────

type memorySnapshotCRUD struct {
	mu        sync.Mutex
	snapshots []history.Snapshot
}

func newMemorySnapshotCRUD() *memorySnapshotCRUD {
	return &memorySnapshotCRUD{}
}

func (s *memorySnapshotCRUD) Create(_ context.Context, snapshot history.Snapshot) (history.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = append(s.snapshots, snapshot)
	return snapshot, nil
}

func (s *memorySnapshotCRUD) Get(_ context.Context, id string, _ ...r3.Query) (history.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, snap := range s.snapshots {
		if snap.ID == id {
			return snap, nil
		}
	}
	return history.Snapshot{}, errors.New("snapshot not found")
}

func (s *memorySnapshotCRUD) Count(ctx context.Context, qargs ...r3.Query) (int64, error) {
	_, n, err := s.List(ctx, qargs...)
	return n, err
}

func (s *memorySnapshotCRUD) List(_ context.Context, qargs ...r3.Query) ([]history.Snapshot, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]history.Snapshot, len(s.snapshots))
	copy(result, s.snapshots)

	// Apply filters
	if len(qargs) > 0 {
		q := qargs[0]
		result = filterSnapshots(result, q.Filters)

		// Apply sorts
		for _, ss := range q.Sorts {
			col := ss.Column.String()
			if col == "version" {
				if ss.Direction == r3.SortDirectionDesc {
					sort.Slice(result, func(i, j int) bool { return result[i].Version > result[j].Version })
				} else {
					sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
				}
			}
		}

		// Apply pagination
		if q.Pagination != nil && q.Pagination.PageSize.Some() {
			limit := q.Pagination.PageSize.Unwrap()
			if limit > 0 && limit < len(result) {
				result = result[:limit]
			}
		}
	}

	return result, int64(len(result)), nil
}

func (s *memorySnapshotCRUD) Update(_ context.Context, snapshot history.Snapshot) (history.Snapshot, error) {
	return snapshot, nil
}

func (s *memorySnapshotCRUD) Patch(
	_ context.Context,
	snapshot history.Snapshot,
	_ r3.Fields,
) (history.Snapshot, error) {
	return snapshot, nil
}

func (s *memorySnapshotCRUD) Delete(_ context.Context, _ string) error {
	return nil
}

func filterSnapshots(snapshots []history.Snapshot, filters r3.Filters) []history.Snapshot {
	if len(filters) == 0 {
		return snapshots
	}
	var result []history.Snapshot
	for _, snap := range snapshots {
		if matchesSnapshotFilters(snap, filters) {
			result = append(result, snap)
		}
	}
	return result
}

func matchesSnapshotFilters(snap history.Snapshot, filters r3.Filters) bool {
	for _, f := range filters {
		if !matchesSnapshotFilter(snap, f) {
			return false
		}
	}
	return true
}

func matchesSnapshotFilter(snap history.Snapshot, f *r3.FilterSpec) bool {
	if len(f.And) > 0 {
		for _, child := range f.And {
			if !matchesSnapshotFilter(snap, child) {
				return false
			}
		}
		return true
	}
	if len(f.Or) > 0 {
		for _, child := range f.Or {
			if matchesSnapshotFilter(snap, child) {
				return true
			}
		}
		return false
	}
	if f.Field == nil {
		return true
	}
	fieldName := f.Field.String()
	val := f.Value
	switch fieldName {
	case "record_type":
		return snap.RecordType == fmt.Sprint(val)
	case "record_id":
		return snap.RecordID == fmt.Sprint(val)
	case "version":
		switch v := val.(type) {
		case int64:
			return snap.Version == v
		case int:
			return snap.Version == int64(v)
		default:
			return strconv.FormatInt(snap.Version, 10) == fmt.Sprint(val)
		}
	default:
		return false
	}
}

// listSnapshots is a helper that queries all snapshots for an entity by recordID.
func listSnapshots(
	ctx context.Context,
	store r3.CRUD[history.Snapshot, string],
	recordID string,
) ([]history.Snapshot, int64, error) {
	q := history.QueryListSnapshots("orders", recordID)
	return store.List(ctx, q)
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestCRUD_CreateRecordsHistory(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, err := repo.Create(ctx, Order{Name: "Test Order", Total: 100, Status: "pending"})
	be.NoError(t, err)

	be.RequireThat(t, order.ID, be.NonZero())

	// Check history was recorded
	records, count, err := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.NoError(t, err)

	be.RequireThat(t, count, be.Eq(int64(1)))

	rec := records[0]
	be.AssertThat(t, rec.Action, be.Eq(history.ActionCreate))
	be.AssertThat(t, rec.Version, be.Eq(int64(1)))
	be.AssertThat(t, rec.Changes.Val, be.NotEmpty())

	// Verify create changes contain all fields with nil OldValue
	for _, c := range rec.Changes.Val {
		be.AssertThat(t, c.OldValue, be.Nil())
	}
}

func TestCRUD_UpdateRecordsDiff(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "pending"})

	// Update
	order.Total = 200
	order.Status = "confirmed"
	updated, err := repo.Update(ctx, order)
	be.NoError(t, err)

	be.AssertThat(t, updated.Total, be.Eq(200))

	// Check history
	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, count, be.Eq(int64(2)))

	updateRec := records[1]
	be.AssertThat(t, updateRec.Action, be.Eq(history.ActionUpdate))
	be.AssertThat(t, updateRec.Version, be.Eq(int64(2)))

	// Check that changes include the diff
	changeMap := make(map[string]history.FieldChange)
	for _, c := range updateRec.Changes.Val {
		changeMap[c.Field] = c
	}

	_, ok := changeMap["total"]
	be.AssertThat(t, ok, be.True())
	_, ok = changeMap["status"]
	be.AssertThat(t, ok, be.True())
}

func TestCRUD_PatchRecordsDiff(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "pending"})

	// Patch only "status"
	order.Status = "shipped"
	_, err := repo.Patch(ctx, order, r3.Fields{r3.NewFieldSpec("status")})
	be.NoError(t, err)

	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, count, be.Eq(int64(2)))

	patchRec := records[1]
	be.AssertThat(t, patchRec.Action, be.Eq(history.ActionPatch))

	// Patch should only record changes for the patched fields
	be.RequireThat(t, patchRec.Changes.Val, be.HaveLength(1))
	be.AssertThat(t, patchRec.Changes.Val[0].Field, be.Eq("status"))
}

func TestCRUD_DeleteRecordsHistory(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Doomed", Total: 50, Status: "pending"})

	err := repo.Delete(ctx, order.ID)
	be.NoError(t, err)

	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, count, be.Eq(int64(2)))

	deleteRec := records[1]
	be.AssertThat(t, deleteRec.Action, be.Eq(history.ActionDelete))
	// Delete should have changes with nil NewValue for each field
	be.AssertThat(t, deleteRec.Changes.Val, be.NotEmpty())
	for _, c := range deleteRec.Changes.Val {
		be.AssertThat(t, c.NewValue, be.Nil())
	}
}

func TestCRUD_ReadsDontRecordHistory(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	// Get and List should not create history entries
	_, _ = repo.Get(ctx, order.ID)
	_, _, _ = repo.List(ctx)

	_, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, count, be.Eq(int64(1)))
}

func TestCRUD_MetadataFunc(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithMetadataFunc[Order, int64](func(_ context.Context) history.Metadata {
			return history.Metadata{
				Source: "api",
			}
		}),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, records, be.NotEmpty())

	be.AssertThat(t, records[0].Metadata.Val.Source, be.Eq("api"))
}

func TestCRUD_ActorContext_AutoPopulate(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// No MetadataFunc — Actor should be auto-populated from context.
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "42", Type: "user"})
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, records, be.NotEmpty())

	be.AssertThat(t, records[0].ActorID, be.Eq("42"))
	be.AssertThat(t, records[0].ActorType, be.Eq("user"))
}

func TestCRUD_RecordEvent(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "5", Type: "user"})
	order, _ := repo.Create(ctx, Order{Name: "X", Total: 1, Status: "ok"})

	ev, err := repo.RecordEvent(ctx, order.ID, "comment_added", map[string]any{"text": "hi"})
	be.NoError(t, err)

	be.AssertThat(t, ev.Action, be.Eq(history.ActionEvent))
	be.AssertThat(t, ev.EventType, be.Eq("comment_added"))
	be.AssertThat(t, ev.EventData.Val["text"], be.Eq("hi"))
	// The event shares the timeline: create (v1) + event (v2).
	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, records, be.HaveLength(2))
}

func TestCRUD_RecordSyntheticCreate(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithFixedActor[Order, int64](r3.Actor{ID: "1", Type: "system"}),
	)

	born := time.Date(2025, 12, 1, 9, 0, 0, 0, time.UTC)
	old := Order{ID: 7, Name: "Old", Total: 9, Status: "ok"}
	rec, err := repo.RecordSyntheticCreate(context.Background(), old, born)
	be.NoError(t, err)

	be.AssertThat(t, rec.Action, be.Eq(history.ActionCreate))
	be.AssertThat(t, rec.Synthetic, be.True())
	be.AssertThat(t, rec.CreatedAt, be_time.SameExactSecond(born))
	be.AssertThat(t, rec.ActorID, be.Eq("1"))
	be.AssertThat(t, rec.Note, be_string.NonEmptyString())
	be.AssertThat(t, rec.Changes.Val, be.NotEmpty())
}

func TestCRUD_FixedActor(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// A system/worker repo: every change is attributed to the fixed actor,
	// ignoring whatever (if anything) is on the context.
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithFixedActor[Order, int64](r3.Actor{ID: "1", Type: "system"}),
	)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "99", Type: "user"})
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 1, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, records, be.NotEmpty())

	be.AssertThat(t, records[0].ActorID, be.Eq("1"))
	be.AssertThat(t, records[0].ActorType, be.Eq("system"))
}

func TestCRUD_ActorContext_SystemDefault(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// No MetadataFunc, no Actor in context — should get SystemActor defaults.
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, records, be.NotEmpty())

	be.AssertThat(t, records[0].ActorID, be_string.EmptyString())
	be.AssertThat(t, records[0].ActorType, be.Eq("system"))
}

func TestCRUD_ActorContext_IsSourceOfTruth(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// MetadataFunc supplies only surrounding context (Source); the actor always
	// comes from r3.GetActor(ctx) and lands on the first-class ChangeRecord
	// fields — MetadataFunc can no longer override it.
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithMetadataFunc[Order, int64](func(_ context.Context) history.Metadata {
			return history.Metadata{
				Source: "admin_ui",
			}
		}),
	)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "7", Type: "admin"})
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, records, be.NotEmpty())

	be.AssertThat(t, records[0].ActorID, be.Eq("7"))
	be.AssertThat(t, records[0].ActorType, be.Eq("admin"))
	be.AssertThat(t, records[0].Metadata.Val.Source, be.Eq("admin_ui"))
}

func TestReverter_ActorContext(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// No MetadataFunc — Actor from context should be used for revert records too.
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "admin_1", Type: "admin"})
	order, _ := repo.Create(ctx, Order{Name: "V1", Total: 100, Status: "a"})
	order.Name = "V2"
	order, _ = repo.Update(ctx, order)

	// Revert to v1
	_, err := repo.Reverter().RevertTo(ctx, order.ID, 1)
	be.NoError(t, err)

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	// Should have 3 records: create + update + revert.
	be.RequireThat(t, records, be.HaveLength(3))

	// All records should have the Actor info.
	for _, rec := range records {
		be.AssertThat(t, rec.ActorID, be.Eq("admin_1"))
		be.AssertThat(t, rec.ActorType, be.Eq("admin"))
	}

	// The revert record should also have extra metadata.
	revertRec := records[2]
	be.AssertThat(t, revertRec.Metadata.Val.Extra["reverted_to_version"], be.Eq("1"))
}

func TestCRUD_ChangesOnlyNoSnapshots(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// Default mode: changes only, no snapshots on ChangeRecord
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	// Create should have changes (every field with nil OldValue)
	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.AssertThat(t, records[0].Changes.Val, be.NotEmpty())

	// Update should have changes (only changed fields)
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	records, _, _ = listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	updateRec := records[1]
	be.AssertThat(t, updateRec.Changes.Val, be.NotEmpty())
}

func TestCRUD_ParentRef(t *testing.T) {
	type Location struct {
		ID     int64  `db:"id,pk"   json:"id"`
		CityID int64  `db:"city_id" json:"city_id"`
		Name   string `db:"name"    json:"name"`
	}

	crud := &mockCRUD[Location, int64]{
		data:   make(map[int64]Location),
		nextID: 1,
	}
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Location, int64](crud, store,
		history.WithIDFunc[Location, int64](func(l Location) int64 { return l.ID }),
		history.WithRecordType[Location, int64]("locations"),
		history.WithParentRef[Location, int64]("cities", "CityID"),
	)

	ctx := context.Background()
	loc, _ := repo.Create(ctx, Location{CityID: 5, Name: "Test Location"})

	records, _, _ := listForRecord(ctx, store, "locations", strconv.FormatInt(loc.ID, 10))
	be.RequireThat(t, records, be.NotEmpty())

	rec := records[0]
	be.AssertThat(t, rec.ParentType, be.Eq("cities"))
	be.AssertThat(t, rec.ParentID, be.Eq("5"))

	// Now test ForTree query
	treeQuery := history.QueryForTree([]history.TreeScope{
		{RecordType: "cities", RecordID: "5"},
		{RecordType: "locations", ParentType: "cities", ParentID: "5"},
	})
	_, count, _ := store.List(ctx, treeQuery)
	be.RequireThat(t, count, be.Eq(int64(1)))
}

func TestReverter_RevertTo(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// Pure diff-based revert: no snapshots, reconstruct from v1 forward
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()

	// Create
	order, _ := repo.Create(ctx, Order{Name: "Original", Total: 100, Status: "pending"})

	// Update to change state
	order.Name = "Modified"
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	// Revert to version 1 (reconstruct from diffs)
	reverted, err := repo.Reverter().RevertTo(ctx, order.ID, 1)
	be.NoError(t, err)

	be.AssertThat(t, reverted.Name, be.Eq("Original"))
	be.AssertThat(t, reverted.Total, be.Eq(100))

	// Check that the revert itself was recorded
	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, count, be.Eq(int64(3)))

	revertRec := records[2]
	be.AssertThat(t, revertRec.Action, be.Eq(history.ActionRevert))
}

func TestReverter_RevertTo_ViaChangeReplay(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// Pure diff replay: v1 create provides all fields, replay v2 changes on top
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()

	// Create (v1, has all fields as changes)
	order, _ := repo.Create(ctx, Order{Name: "V1", Total: 100, Status: "pending"})

	// Update (v2, changes-only: name V1->V2, total 100->200)
	order.Name = "V2"
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	// Update (v3, changes-only: name V2->V3, status pending->shipped)
	order.Name = "V3"
	order.Status = "shipped"
	order, _ = repo.Update(ctx, order)

	// Revert to v2 — must reconstruct from v1 changes + v2 changes
	reverted, err := repo.Reverter().RevertTo(ctx, order.ID, 2)
	be.NoError(t, err)

	be.AssertThat(t, reverted.Name, be.Eq("V2"))
	be.AssertThat(t, reverted.Total, be.Eq(200))
	be.AssertThat(t, reverted.Status, be.Eq("pending"))
}

func TestReverter_Reconstruct(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()

	// Build a chain: v1 (create) -> v2 (changes) -> v3 (changes) -> v4 (changes)
	order, _ := repo.Create(ctx, Order{Name: "V1", Total: 100, Status: "a"})

	order.Name = "V2"
	order, _ = repo.Update(ctx, order)

	order.Total = 300
	order, _ = repo.Update(ctx, order)

	order.Status = "b"
	order, _ = repo.Update(ctx, order)

	recordID := strconv.FormatInt(order.ID, 10)
	reverter := repo.Reverter()

	// Reconstruct v1 (from create changes)
	v1, err := reverter.Reconstruct(ctx, recordID, 1)
	be.NoError(t, err)

	be.AssertThat(t, v1.Name, be.Eq("V1"))
	be.AssertThat(t, v1.Total, be.Eq(100))
	be.AssertThat(t, v1.Status, be.Eq("a"))

	// Reconstruct v2 (replay v1 + v2 changes)
	v2, err := reverter.Reconstruct(ctx, recordID, 2)
	be.NoError(t, err)

	be.AssertThat(t, v2.Name, be.Eq("V2"))
	be.AssertThat(t, v2.Total, be.Eq(100))
	be.AssertThat(t, v2.Status, be.Eq("a"))

	// Reconstruct v3 (replay v1+v2+v3 changes)
	v3, err := reverter.Reconstruct(ctx, recordID, 3)
	be.NoError(t, err)

	be.AssertThat(t, v3.Name, be.Eq("V2"))
	be.AssertThat(t, v3.Total, be.Eq(300))
	be.AssertThat(t, v3.Status, be.Eq("a"))

	// Reconstruct v4 (replay all changes)
	v4, err := reverter.Reconstruct(ctx, recordID, 4)
	be.NoError(t, err)

	be.AssertThat(t, v4.Name, be.Eq("V2"))
	be.AssertThat(t, v4.Total, be.Eq(300))
	be.AssertThat(t, v4.Status, be.Eq("b"))
}

func TestReverter_RevertLast(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()

	// Create -> Update -> Update
	order, _ := repo.Create(ctx, Order{Name: "V1", Total: 100, Status: "a"})
	order.Name = "V2"
	order, _ = repo.Update(ctx, order)
	order.Name = "V3"
	order, _ = repo.Update(ctx, order)

	// RevertLast should go back to V2 (reconstructed from v1+v2 changes)
	reverted, err := repo.Reverter().RevertLast(ctx, order.ID)
	be.NoError(t, err)

	be.AssertThat(t, reverted.Name, be.Eq("V2"))
	be.AssertThat(t, reverted.Total, be.Eq(100))
}

func TestCRUD_DeriveRecordType(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// Don't set RecordType — should auto-derive from Order -> "orders"
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 1, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, records, be.NotEmpty())

	be.AssertThat(t, records[0].RecordType, be.Eq("orders"))
}

func TestCRUD_SnapshotRules(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()
	snapStore := newMemorySnapshotCRUD()

	// Snapshot rule: take a snapshot when status changes to "published"
	rule := history.SnapshotRule[Order]{
		Name:  "on_publish",
		Store: snapStore,
		Condition: func(_ history.Action, old, cur Order) bool {
			return old.Status != "published" && cur.Status == "published"
		},
	}

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithSnapshotRules[Order, int64](rule),
	)

	ctx := context.Background()

	// Create with status "draft" — should NOT trigger snapshot
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "draft"})

	snaps, _, _ := listSnapshots(ctx, snapStore, strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, snaps, be.Empty())

	// Update status to "published" — should trigger snapshot
	order.Status = "published"
	order, _ = repo.Update(ctx, order)

	snaps, _, _ = listSnapshots(ctx, snapStore, strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, snaps, be.HaveLength(1))

	snap := snaps[0]
	be.AssertThat(t, snap.RuleName, be.Eq("on_publish"))
	be.AssertThat(t, snap.RecordType, be.Eq("orders"))

	// Verify snapshot data deserializes correctly
	restored, err := history.UnmarshalSnapshot[Order](snap.Data)
	be.NoError(t, err)

	be.AssertThat(t, restored.Status, be.Eq("published"))
	be.AssertThat(t, restored.Name, be.Eq("Test"))

	// Another update (not changing to published) — should NOT trigger another snapshot
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	snaps, _, _ = listSnapshots(ctx, snapStore, strconv.FormatInt(order.ID, 10))
	be.RequireThat(t, snaps, be.HaveLength(1))
}

func TestCRUD_SnapshotRules_MultipleRules(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()
	snapStore1 := newMemorySnapshotCRUD()
	snapStore2 := newMemorySnapshotCRUD()

	// Rule 1: snapshot on publish
	rule1 := history.SnapshotRule[Order]{
		Name:  "on_publish",
		Store: snapStore1,
		Condition: func(_ history.Action, old, cur Order) bool {
			return old.Status != "published" && cur.Status == "published"
		},
	}

	// Rule 2: snapshot on high-value orders (total >= 1000)
	rule2 := history.SnapshotRule[Order]{
		Name:  "high_value",
		Store: snapStore2,
		Condition: func(action history.Action, _, cur Order) bool {
			return action == history.ActionCreate && cur.Total >= 1000
		},
	}

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithSnapshotRules[Order, int64](rule1, rule2),
	)

	ctx := context.Background()

	// Create a high-value order with draft status
	order, _ := repo.Create(ctx, Order{Name: "Big Order", Total: 5000, Status: "draft"})

	// Rule 1 should NOT fire (not published), Rule 2 SHOULD fire (high value create)
	snaps1, _, _ := listSnapshots(ctx, snapStore1, strconv.FormatInt(order.ID, 10))
	snaps2, _, _ := listSnapshots(ctx, snapStore2, strconv.FormatInt(order.ID, 10))

	be.AssertThat(t, snaps1, be.Empty())
	be.AssertThat(t, snaps2, be.HaveLength(1))

	// Now publish — Rule 1 fires, Rule 2 does not (not a create)
	order.Status = "published"
	order, _ = repo.Update(ctx, order)

	snaps1, _, _ = listSnapshots(ctx, snapStore1, strconv.FormatInt(order.ID, 10))
	snaps2, _, _ = listSnapshots(ctx, snapStore2, strconv.FormatInt(order.ID, 10))

	be.AssertThat(t, snaps1, be.HaveLength(1))
	be.AssertThat(t, snaps2, be.HaveLength(1))
}

// TestHistory_QueryBuilders tests that the query builders return correct results
// when used with the in-memory CRUD.
func TestHistory_QueryBuilders(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	recordID := strconv.FormatInt(order.ID, 10)

	// QueryForRecord
	records, _, err := store.List(ctx, history.QueryForRecord("orders", recordID))
	be.NoError(t, err)

	be.AssertThat(t, records, be.HaveLength(2))
	// Should be sorted by version ascending
	be.AssertThat(t, records[0].Version, be.Lte(records[1].Version))

	// QueryForType
	records, _, err = store.List(ctx, history.QueryForType("orders"))
	be.NoError(t, err)

	be.AssertThat(t, records, be.HaveLength(2))

	// QueryLatestVersion
	records, _, err = store.List(ctx, history.QueryLatestVersion("orders", recordID))
	be.NoError(t, err)

	be.RequireThat(t, records, be.HaveLength(1))
	be.AssertThat(t, records[0].Version, be.Eq(int64(2)))
}

// ── Retention Tests ──────────────────────────────────────────────────────

func TestRetentionEnforcer_MaxAge(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()

	// Create several changes over "time" (we fake timestamps via direct store access).
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "a"})
	order.Total = 200
	order, _ = repo.Update(ctx, order)
	order.Total = 300
	order, _ = repo.Update(ctx, order)

	recordID := strconv.FormatInt(order.ID, 10)

	// Verify we have 3 records.
	records, _, _ := listForRecord(ctx, store, "orders", recordID)
	be.RequireThat(t, records, be.HaveLength(3))

	// The enforcer with a very short MaxAge should delete nothing since records are fresh.
	enforcer := history.NewRetentionEnforcer(store, history.RetentionPolicy{
		MaxAge: 24 * time.Hour,
	})
	deleted := enforcer.Enforce(ctx, "orders")
	be.AssertThat(t, deleted, be.Eq(int64(0)))
}

func TestRetentionEnforcer_MaxVersions(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()

	// Create 5 versions.
	order, _ := repo.Create(ctx, Order{Name: "V1", Total: 100, Status: "a"})
	for i := range 4 {
		order.Name = fmt.Sprintf("V%d", i+2)
		order, _ = repo.Update(ctx, order)
	}

	recordID := strconv.FormatInt(order.ID, 10)
	records, _, _ := listForRecord(ctx, store, "orders", recordID)
	be.RequireThat(t, records, be.HaveLength(5))

	// Keep only 3 versions.
	enforcer := history.NewRetentionEnforcer(store, history.RetentionPolicy{
		MaxVersions: 3,
	})
	deleted := enforcer.Enforce(ctx, "orders")
	be.AssertThat(t, deleted, be.Eq(int64(2)))

	// Remaining should be the latest 3.
	remaining, _, _ := listForRecord(ctx, store, "orders", recordID)
	be.RequireThat(t, remaining, be.HaveLength(3))
	// Oldest remaining should be version 3.
	be.AssertThat(t, remaining[0].Version, be.Eq(int64(3)))
}

// ── Generic mock CRUD for testing with different types ────────────────

type mockCRUD[T any, ID comparable] struct {
	mu     sync.Mutex
	data   map[ID]T
	nextID ID
}

func (m *mockCRUD[T, ID]) Create(_ context.Context, entity T) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Use nextID and increment (only works for int64-like types)
	id := m.nextID
	// This is a simplification — in real code you'd set the ID on the entity
	m.data[id] = entity
	// Increment nextID via interface (works for int64)
	m.nextID = incrementID(m.nextID)
	return entity, nil
}

func (m *mockCRUD[T, ID]) Get(_ context.Context, id ID, _ ...r3.Query) (T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity, ok := m.data[id]
	if !ok {
		var zero T
		return zero, errors.New("not found")
	}
	return entity, nil
}

func (m *mockCRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, qarg...)
	return n, err
}

func (m *mockCRUD[T, ID]) List(_ context.Context, _ ...r3.Query) ([]T, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []T
	for _, v := range m.data {
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *mockCRUD[T, ID]) Update(_ context.Context, entity T) (T, error) {
	return entity, nil
}

func (m *mockCRUD[T, ID]) Patch(_ context.Context, entity T, _ r3.Fields) (T, error) {
	return entity, nil
}

func (m *mockCRUD[T, ID]) Delete(_ context.Context, _ ID) error {
	return nil
}

func incrementID[ID comparable](id ID) ID {
	// Type switch for incrementing
	switch v := any(id).(type) {
	case int64:
		return any(v + 1).(ID)
	case int:
		return any(v + 1).(ID)
	default:
		return id
	}
}

// TestCRUD_ConcurrentUpdatesGetUniqueVersions verifies that concurrent
// mutations of the same record are assigned unique, contiguous versions and
// unique IDs (C3). Before the fix, the unlocked read-modify-write in
// nextVersion handed out duplicate versions, and time-based IDs could collide.
func TestCRUD_ConcurrentUpdatesGetUniqueVersions(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()
	repo := history.WithHistory[Order, int64](inner, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)
	ctx := context.Background()

	created, err := repo.Create(ctx, Order{Name: "init"})
	be.NoError(t, err)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			_, err := repo.Update(ctx, Order{ID: created.ID, Name: fmt.Sprintf("v%d", i)})
			be.AssertThat(t, err, be.Succeed())
		}(i)
	}
	wg.Wait()

	records, _, err := store.List(ctx)
	be.NoError(t, err)

	// 1 create + n updates.
	wantCount := n + 1
	be.RequireThat(t, records, be.HaveLength(wantCount))

	versions := make(map[int64]int, wantCount)
	ids := make(map[string]int, wantCount)
	for _, r := range records {
		versions[r.Version]++
		be.AssertThat(t, r.ID, be_string.NonEmptyString())
		ids[r.ID]++
	}

	// Versions must be exactly 1..wantCount, each assigned once.
	for v := int64(1); v <= int64(wantCount); v++ {
		be.AssertThat(t, versions[v], be.Eq(1))
	}
	// IDs must be unique.
	for _, c := range ids {
		be.AssertThat(t, c, be.Eq(1))
	}
}

// failingChangeRecordStore is a change-record store whose Create always fails,
// used to exercise the audit-write failure policy (H10). List/Get/etc. are
// inherited so nextVersion still works.
type failingChangeRecordStore struct {
	*memoryChangeRecordCRUD
}

func (s *failingChangeRecordStore) Create(context.Context, history.ChangeRecord) (history.ChangeRecord, error) {
	return history.ChangeRecord{}, errors.New("change store write failed")
}

// TestCRUD_SyncRecordErrorPolicy verifies the synchronous audit-write failure
// policy (H10): by default the operation succeeds and the error is reported via
// ErrorHandler (best-effort); with WithFailOnError the error is surfaced to the
// caller even though the mutation was already applied.
func TestCRUD_SyncRecordErrorPolicy(t *testing.T) {
	idFn := history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID })

	t.Run("default is best-effort and reports via ErrorHandler", func(t *testing.T) {
		inner := newMemoryCRUD()
		store := &failingChangeRecordStore{memoryChangeRecordCRUD: newMemoryChangeRecordCRUD()}

		var handled error
		repo := history.WithHistory[Order, int64](inner, store, idFn,
			history.WithErrorHandler[Order, int64](func(err error) { handled = err }),
		)

		created, err := repo.Create(context.Background(), Order{Name: "x"})
		be.NoError(t, err)

		be.AssertThat(t, created.ID, be.NonZero())
		be.AssertThat(t, handled, be.HaveOccurred())
	})

	t.Run("FailOnError surfaces the audit error", func(t *testing.T) {
		inner := newMemoryCRUD()
		store := &failingChangeRecordStore{memoryChangeRecordCRUD: newMemoryChangeRecordCRUD()}

		repo := history.WithHistory[Order, int64](inner, store, idFn,
			history.WithFailOnError[Order, int64](),
		)

		_, err := repo.Create(context.Background(), Order{Name: "y"})
		be.Error(t, err)

		// The mutation was already applied before the audit attempt.
		be.AssertThat(t, len(inner.data), be.Eq(1))
	})
}
