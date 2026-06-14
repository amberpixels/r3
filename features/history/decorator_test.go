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
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if order.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	// Check history was recorded
	records, count, err := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 record, got %d", count)
	}

	rec := records[0]
	if rec.Action != history.ActionCreate {
		t.Errorf("expected action 'create', got %q", rec.Action)
	}
	if rec.Version != 1 {
		t.Errorf("expected version 1, got %d", rec.Version)
	}
	if len(rec.Changes.Val) == 0 {
		t.Error("expected non-empty changes for create")
	}

	// Verify create changes contain all fields with nil OldValue
	for _, c := range rec.Changes.Val {
		if c.OldValue != nil {
			t.Errorf("create change for %s should have nil OldValue, got %v", c.Field, c.OldValue)
		}
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
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Total != 200 {
		t.Errorf("expected Total=200, got %d", updated.Total)
	}

	// Check history
	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	updateRec := records[1]
	if updateRec.Action != history.ActionUpdate {
		t.Errorf("expected action 'update', got %q", updateRec.Action)
	}
	if updateRec.Version != 2 {
		t.Errorf("expected version 2, got %d", updateRec.Version)
	}

	// Check that changes include the diff
	changeMap := make(map[string]history.FieldChange)
	for _, c := range updateRec.Changes.Val {
		changeMap[c.Field] = c
	}

	if _, ok := changeMap["total"]; !ok {
		t.Error("expected 'total' in changes")
	}
	if _, ok := changeMap["status"]; !ok {
		t.Error("expected 'status' in changes")
	}
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
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}

	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	patchRec := records[1]
	if patchRec.Action != history.ActionPatch {
		t.Errorf("expected action 'patch', got %q", patchRec.Action)
	}

	// Patch should only record changes for the patched fields
	if len(patchRec.Changes.Val) != 1 {
		t.Fatalf("expected 1 change for patch, got %d: %v", len(patchRec.Changes.Val), patchRec.Changes.Val)
	}
	if patchRec.Changes.Val[0].Field != "status" {
		t.Errorf("expected field 'status', got %q", patchRec.Changes.Val[0].Field)
	}
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
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	deleteRec := records[1]
	if deleteRec.Action != history.ActionDelete {
		t.Errorf("expected action 'delete', got %q", deleteRec.Action)
	}
	// Delete should have changes with nil NewValue for each field
	if len(deleteRec.Changes.Val) == 0 {
		t.Error("expected non-empty changes for delete")
	}
	for _, c := range deleteRec.Changes.Val {
		if c.NewValue != nil {
			t.Errorf("delete change for %s should have nil NewValue, got %v", c.Field, c.NewValue)
		}
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

	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if count != 1 {
		t.Fatalf("expected 1 record (only create), got %d: %v", count, records)
	}
}

func TestCRUD_MetadataFunc(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithMetadataFunc[Order, int64](func(_ context.Context) history.Metadata {
			return history.Metadata{
				ActorID:   "user_42",
				ActorType: "user",
				Source:    "api",
			}
		}),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	meta := records[0].Metadata.Val
	if meta.ActorID != "user_42" {
		t.Errorf("expected ActorID 'user_42', got %q", meta.ActorID)
	}
	if meta.Source != "api" {
		t.Errorf("expected Source 'api', got %q", meta.Source)
	}
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
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	meta := records[0].Metadata.Val
	if meta.ActorID != "42" {
		t.Errorf("expected ActorID '42', got %q", meta.ActorID)
	}
	if meta.ActorType != "user" {
		t.Errorf("expected ActorType 'user', got %q", meta.ActorType)
	}
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
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	meta := records[0].Metadata.Val
	if meta.ActorID != "" {
		t.Errorf("expected empty ActorID for SystemActor, got %q", meta.ActorID)
	}
	if meta.ActorType != "system" {
		t.Errorf("expected ActorType 'system', got %q", meta.ActorType)
	}
}

func TestCRUD_ActorContext_MetadataFuncTakesPrecedence(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// MetadataFunc explicitly sets ActorID/ActorType — these should win over context.
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
		history.WithMetadataFunc[Order, int64](func(_ context.Context) history.Metadata {
			return history.Metadata{
				ActorID:   "api_key_99",
				ActorType: "api_key",
				Source:    "api",
			}
		}),
	)

	// Context has a different actor — should be overridden by MetadataFunc.
	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "42", Type: "user"})
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	meta := records[0].Metadata.Val
	if meta.ActorID != "api_key_99" {
		t.Errorf("expected ActorID 'api_key_99', got %q", meta.ActorID)
	}
	if meta.ActorType != "api_key" {
		t.Errorf("expected ActorType 'api_key', got %q", meta.ActorType)
	}
	if meta.Source != "api" {
		t.Errorf("expected Source 'api', got %q", meta.Source)
	}
}

func TestCRUD_ActorContext_MetadataFuncPartialFillFromContext(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryChangeRecordCRUD()

	// MetadataFunc sets Source but leaves ActorID/ActorType empty — should be filled from context.
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
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	meta := records[0].Metadata.Val
	if meta.ActorID != "7" {
		t.Errorf("expected ActorID '7' from context, got %q", meta.ActorID)
	}
	if meta.ActorType != "admin" {
		t.Errorf("expected ActorType 'admin' from context, got %q", meta.ActorType)
	}
	if meta.Source != "admin_ui" {
		t.Errorf("expected Source 'admin_ui' from MetadataFunc, got %q", meta.Source)
	}
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
	if err != nil {
		t.Fatalf("RevertTo failed: %v", err)
	}

	records, _, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	// Should have 3 records: create + update + revert.
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// All records should have the Actor info.
	for _, rec := range records {
		meta := rec.Metadata.Val
		if meta.ActorID != "admin_1" {
			t.Errorf("record %s: expected ActorID 'admin_1', got %q", rec.Action, meta.ActorID)
		}
		if meta.ActorType != "admin" {
			t.Errorf("record %s: expected ActorType 'admin', got %q", rec.Action, meta.ActorType)
		}
	}

	// The revert record should also have extra metadata.
	revertRec := records[2]
	if revertRec.Metadata.Val.Extra["reverted_to_version"] != "1" {
		t.Errorf("expected reverted_to_version '1', got %q", revertRec.Metadata.Val.Extra["reverted_to_version"])
	}
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
	if len(records[0].Changes.Val) == 0 {
		t.Error("create should have non-empty changes (all fields)")
	}

	// Update should have changes (only changed fields)
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	records, _, _ = listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	updateRec := records[1]
	if len(updateRec.Changes.Val) == 0 {
		t.Error("update should have non-empty changes")
	}
}

func TestCRUD_ParentRef(t *testing.T) {
	type Adset struct {
		ID         int64  `db:"id,pk"       json:"id"`
		CampaignID int64  `db:"campaign_id" json:"campaign_id"`
		Name       string `db:"name"        json:"name"`
	}

	crud := &mockCRUD[Adset, int64]{
		data:   make(map[int64]Adset),
		nextID: 1,
	}
	store := newMemoryChangeRecordCRUD()

	repo := history.WithHistory[Adset, int64](crud, store,
		history.WithIDFunc[Adset, int64](func(a Adset) int64 { return a.ID }),
		history.WithRecordType[Adset, int64]("adsets"),
		history.WithParentRef[Adset, int64]("campaigns", "CampaignID"),
	)

	ctx := context.Background()
	adset, _ := repo.Create(ctx, Adset{CampaignID: 5, Name: "Test Adset"})

	records, _, _ := listForRecord(ctx, store, "adsets", strconv.FormatInt(adset.ID, 10))
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	rec := records[0]
	if rec.ParentType != "campaigns" {
		t.Errorf("expected ParentType 'campaigns', got %q", rec.ParentType)
	}
	if rec.ParentID != "5" {
		t.Errorf("expected ParentID '5', got %q", rec.ParentID)
	}

	// Now test ForTree query
	treeQuery := history.QueryForTree([]history.TreeScope{
		{RecordType: "campaigns", RecordID: "5"},
		{RecordType: "adsets", ParentType: "campaigns", ParentID: "5"},
	})
	treeRecords, count, _ := store.List(ctx, treeQuery)
	if count != 1 {
		t.Fatalf("expected 1 tree record, got %d: %v", count, treeRecords)
	}
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
	if err != nil {
		t.Fatalf("RevertTo failed: %v", err)
	}

	if reverted.Name != "Original" {
		t.Errorf("expected Name='Original' after revert, got %q", reverted.Name)
	}
	if reverted.Total != 100 {
		t.Errorf("expected Total=100 after revert, got %d", reverted.Total)
	}

	// Check that the revert itself was recorded
	records, count, _ := listForRecord(ctx, store, "orders", strconv.FormatInt(order.ID, 10))
	if count != 3 {
		t.Fatalf("expected 3 records (create + update + revert), got %d", count)
	}

	revertRec := records[2]
	if revertRec.Action != history.ActionRevert {
		t.Errorf("expected action 'revert', got %q", revertRec.Action)
	}
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
	if err != nil {
		t.Fatalf("RevertTo via replay failed: %v", err)
	}

	if reverted.Name != "V2" {
		t.Errorf("expected Name='V2' after replay revert, got %q", reverted.Name)
	}
	if reverted.Total != 200 {
		t.Errorf("expected Total=200 after replay revert, got %d", reverted.Total)
	}
	if reverted.Status != "pending" {
		t.Errorf("expected Status='pending' after replay revert, got %q", reverted.Status)
	}
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
	if err != nil {
		t.Fatalf("Reconstruct v1 failed: %v", err)
	}
	if v1.Name != "V1" || v1.Total != 100 || v1.Status != "a" {
		t.Errorf("v1 mismatch: %+v", v1)
	}

	// Reconstruct v2 (replay v1 + v2 changes)
	v2, err := reverter.Reconstruct(ctx, recordID, 2)
	if err != nil {
		t.Fatalf("Reconstruct v2 failed: %v", err)
	}
	if v2.Name != "V2" || v2.Total != 100 || v2.Status != "a" {
		t.Errorf("v2 mismatch: got %+v", v2)
	}

	// Reconstruct v3 (replay v1+v2+v3 changes)
	v3, err := reverter.Reconstruct(ctx, recordID, 3)
	if err != nil {
		t.Fatalf("Reconstruct v3 failed: %v", err)
	}
	if v3.Name != "V2" || v3.Total != 300 || v3.Status != "a" {
		t.Errorf("v3 mismatch: got %+v", v3)
	}

	// Reconstruct v4 (replay all changes)
	v4, err := reverter.Reconstruct(ctx, recordID, 4)
	if err != nil {
		t.Fatalf("Reconstruct v4 failed: %v", err)
	}
	if v4.Name != "V2" || v4.Total != 300 || v4.Status != "b" {
		t.Errorf("v4 mismatch: got %+v", v4)
	}
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
	if err != nil {
		t.Fatalf("RevertLast failed: %v", err)
	}

	if reverted.Name != "V2" {
		t.Errorf("expected Name='V2' after RevertLast, got %q", reverted.Name)
	}
	if reverted.Total != 100 {
		t.Errorf("expected Total=100 after RevertLast, got %d", reverted.Total)
	}
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
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].RecordType != "orders" {
		t.Errorf("expected RecordType 'orders', got %q", records[0].RecordType)
	}
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
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots after create with draft status, got %d", len(snaps))
	}

	// Update status to "published" — should trigger snapshot
	order.Status = "published"
	order, _ = repo.Update(ctx, order)

	snaps, _, _ = listSnapshots(ctx, snapStore, strconv.FormatInt(order.ID, 10))
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot after publish, got %d", len(snaps))
	}

	snap := snaps[0]
	if snap.RuleName != "on_publish" {
		t.Errorf("expected rule name 'on_publish', got %q", snap.RuleName)
	}
	if snap.RecordType != "orders" {
		t.Errorf("expected record type 'orders', got %q", snap.RecordType)
	}

	// Verify snapshot data deserializes correctly
	restored, err := history.UnmarshalSnapshot[Order](snap.Data)
	if err != nil {
		t.Fatalf("failed to unmarshal snapshot data: %v", err)
	}
	if restored.Status != "published" {
		t.Errorf("expected snapshot status 'published', got %q", restored.Status)
	}
	if restored.Name != "Test" {
		t.Errorf("expected snapshot name 'Test', got %q", restored.Name)
	}

	// Another update (not changing to published) — should NOT trigger another snapshot
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	snaps, _, _ = listSnapshots(ctx, snapStore, strconv.FormatInt(order.ID, 10))
	if len(snaps) != 1 {
		t.Fatalf("expected still 1 snapshot after non-publish update, got %d", len(snaps))
	}
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

	if len(snaps1) != 0 {
		t.Errorf("expected 0 publish snapshots, got %d", len(snaps1))
	}
	if len(snaps2) != 1 {
		t.Errorf("expected 1 high-value snapshot, got %d", len(snaps2))
	}

	// Now publish — Rule 1 fires, Rule 2 does not (not a create)
	order.Status = "published"
	order, _ = repo.Update(ctx, order)

	snaps1, _, _ = listSnapshots(ctx, snapStore1, strconv.FormatInt(order.ID, 10))
	snaps2, _, _ = listSnapshots(ctx, snapStore2, strconv.FormatInt(order.ID, 10))

	if len(snaps1) != 1 {
		t.Errorf("expected 1 publish snapshot, got %d", len(snaps1))
	}
	if len(snaps2) != 1 {
		t.Errorf("expected still 1 high-value snapshot, got %d", len(snaps2))
	}
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
	if err != nil {
		t.Fatalf("QueryForRecord failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
	// Should be sorted by version ascending
	if records[0].Version > records[1].Version {
		t.Error("expected records sorted by version ASC")
	}

	// QueryForType
	records, _, err = store.List(ctx, history.QueryForType("orders"))
	if err != nil {
		t.Fatalf("QueryForType failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records for type, got %d", len(records))
	}

	// QueryLatestVersion
	records, _, err = store.List(ctx, history.QueryLatestVersion("orders", recordID))
	if err != nil {
		t.Fatalf("QueryLatestVersion failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 latest record, got %d", len(records))
	}
	if records[0].Version != 2 {
		t.Errorf("expected latest version 2, got %d", records[0].Version)
	}
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
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// The enforcer with a very short MaxAge should delete nothing since records are fresh.
	enforcer := history.NewRetentionEnforcer(store, history.RetentionPolicy{
		MaxAge: 24 * time.Hour,
	})
	deleted := enforcer.Enforce(ctx, "orders")
	if deleted != 0 {
		t.Errorf("expected 0 deleted (all fresh), got %d", deleted)
	}
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
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}

	// Keep only 3 versions.
	enforcer := history.NewRetentionEnforcer(store, history.RetentionPolicy{
		MaxVersions: 3,
	})
	deleted := enforcer.Enforce(ctx, "orders")
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Remaining should be the latest 3.
	remaining, _, _ := listForRecord(ctx, store, "orders", recordID)
	if len(remaining) != 3 {
		t.Fatalf("expected 3 remaining records, got %d", len(remaining))
	}
	// Oldest remaining should be version 3.
	if remaining[0].Version != 3 {
		t.Errorf("expected oldest remaining version 3, got %d", remaining[0].Version)
	}
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
