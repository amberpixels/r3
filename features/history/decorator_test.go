package history_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/history"
)

// ── In-Memory CRUD (mock) ────────────────────────────────────────────────

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

// ── In-Memory Store (mock) ────────────────────────────────────────────────

type memoryStore struct {
	mu      sync.Mutex
	records []history.ChangeRecord
}

func newMemoryStore() *memoryStore {
	return &memoryStore{}
}

func (s *memoryStore) Record(_ context.Context, record history.ChangeRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *memoryStore) ForRecord(
	_ context.Context,
	recordType string,
	recordID string,
	_ ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []history.ChangeRecord
	for _, r := range s.records {
		if r.RecordType == recordType && r.RecordID == recordID {
			result = append(result, r)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	return result, int64(len(result)), nil
}

func (s *memoryStore) ForType(
	_ context.Context,
	recordType string,
	_ ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []history.ChangeRecord
	for _, r := range s.records {
		if r.RecordType == recordType {
			result = append(result, r)
		}
	}
	return result, int64(len(result)), nil
}

func (s *memoryStore) ForTree(
	_ context.Context,
	scopes []history.TreeScope,
	_ ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []history.ChangeRecord
	for _, r := range s.records {
		for _, scope := range scopes {
			if r.RecordType == scope.RecordType {
				match := true
				if scope.RecordID != "" && r.RecordID != scope.RecordID {
					match = false
				}
				if scope.ParentType != "" && r.ParentType != scope.ParentType {
					match = false
				}
				if scope.ParentID != "" && r.ParentID != scope.ParentID {
					match = false
				}
				if match {
					result = append(result, r)
					break
				}
			}
		}
	}
	return result, int64(len(result)), nil
}

func (s *memoryStore) NextVersion(_ context.Context, recordType string, recordID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var maxVersion int64
	for _, r := range s.records {
		if r.RecordType == recordType && r.RecordID == recordID {
			if r.Version > maxVersion {
				maxVersion = r.Version
			}
		}
	}
	return maxVersion + 1, nil
}

func (s *memoryStore) GetVersion(
	_ context.Context,
	recordType string,
	recordID string,
	version int64,
) (history.ChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.records {
		if r.RecordType == recordType && r.RecordID == recordID && r.Version == version {
			return r, nil
		}
	}
	return history.ChangeRecord{}, history.ErrVersionNotFound
}

func (s *memoryStore) LatestVersion(
	_ context.Context,
	recordType string,
	recordID string,
) (history.ChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest *history.ChangeRecord
	for i, r := range s.records {
		if r.RecordType == recordType && r.RecordID == recordID {
			if latest == nil || r.Version > latest.Version {
				latest = &s.records[i]
			}
		}
	}
	if latest == nil {
		return history.ChangeRecord{}, history.ErrNoHistory
	}
	return *latest, nil
}

// ── In-Memory SnapshotStore (mock) ──────────────────────────────────────

type memorySnapshotStore struct {
	mu        sync.Mutex
	snapshots []history.Snapshot
}

func newMemorySnapshotStore() *memorySnapshotStore {
	return &memorySnapshotStore{}
}

func (s *memorySnapshotStore) SaveSnapshot(_ context.Context, snapshot history.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots = append(s.snapshots, snapshot)
	return nil
}

func (s *memorySnapshotStore) GetSnapshot(
	_ context.Context,
	recordType string,
	recordID string,
	version int64,
) (history.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, snap := range s.snapshots {
		if snap.RecordType == recordType && snap.RecordID == recordID && snap.Version == version {
			return snap, nil
		}
	}
	return history.Snapshot{}, errors.New("snapshot not found")
}

func (s *memorySnapshotStore) ListSnapshots(
	_ context.Context,
	recordType string,
	recordID string,
) ([]history.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []history.Snapshot
	for _, snap := range s.snapshots {
		if snap.RecordType == recordType && snap.RecordID == recordID {
			result = append(result, snap)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version > result[j].Version })
	return result, nil
}

func (s *memorySnapshotStore) LatestSnapshot(
	_ context.Context,
	recordType string,
	recordID string,
) (history.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest *history.Snapshot
	for i, snap := range s.snapshots {
		if snap.RecordType == recordType && snap.RecordID == recordID {
			if latest == nil || snap.Version > latest.Version {
				latest = &s.snapshots[i]
			}
		}
	}
	if latest == nil {
		return history.Snapshot{}, errors.New("no snapshots")
	}
	return *latest, nil
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestCRUD_CreateRecordsHistory(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()

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
	records, count, err := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if err != nil {
		t.Fatalf("ForRecord failed: %v", err)
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
	if len(rec.Changes) == 0 {
		t.Error("expected non-empty changes for create")
	}

	// Verify create changes contain all fields with nil OldValue
	for _, c := range rec.Changes {
		if c.OldValue != nil {
			t.Errorf("create change for %s should have nil OldValue, got %v", c.Field, c.OldValue)
		}
	}
}

func TestCRUD_UpdateRecordsDiff(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()

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
	records, count, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
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
	for _, c := range updateRec.Changes {
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
	store := newMemoryStore()

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

	records, count, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	patchRec := records[1]
	if patchRec.Action != history.ActionPatch {
		t.Errorf("expected action 'patch', got %q", patchRec.Action)
	}

	// Patch should only record changes for the patched fields
	if len(patchRec.Changes) != 1 {
		t.Fatalf("expected 1 change for patch, got %d: %v", len(patchRec.Changes), patchRec.Changes)
	}
	if patchRec.Changes[0].Field != "status" {
		t.Errorf("expected field 'status', got %q", patchRec.Changes[0].Field)
	}
}

func TestCRUD_DeleteRecordsHistory(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Doomed", Total: 50, Status: "pending"})

	err := repo.Delete(ctx, order.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	records, count, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	deleteRec := records[1]
	if deleteRec.Action != history.ActionDelete {
		t.Errorf("expected action 'delete', got %q", deleteRec.Action)
	}
	// Delete should have changes with nil NewValue for each field
	if len(deleteRec.Changes) == 0 {
		t.Error("expected non-empty changes for delete")
	}
	for _, c := range deleteRec.Changes {
		if c.NewValue != nil {
			t.Errorf("delete change for %s should have nil NewValue, got %v", c.Field, c.NewValue)
		}
	}
}

func TestCRUD_ReadsDontRecordHistory(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()

	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	// Get and List should not create history entries
	_, _ = repo.Get(ctx, order.ID)
	_, _, _ = repo.List(ctx)

	records, count, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if count != 1 {
		t.Fatalf("expected 1 record (only create), got %d: %v", count, records)
	}
}

func TestCRUD_MetadataFunc(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()

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

	records, _, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	meta := records[0].Metadata
	if meta.ActorID != "user_42" {
		t.Errorf("expected ActorID 'user_42', got %q", meta.ActorID)
	}
	if meta.Source != "api" {
		t.Errorf("expected Source 'api', got %q", meta.Source)
	}
}

func TestCRUD_ChangesOnlyNoSnapshots(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()

	// Default mode: changes only, no snapshots on ChangeRecord
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "ok"})

	// Create should have changes (every field with nil OldValue)
	records, _, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if len(records[0].Changes) == 0 {
		t.Error("create should have non-empty changes (all fields)")
	}

	// Update should have changes (only changed fields)
	order.Total = 200
	order, _ = repo.Update(ctx, order)

	records, _, _ = store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	updateRec := records[1]
	if len(updateRec.Changes) == 0 {
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
	store := newMemoryStore()

	repo := history.WithHistory[Adset, int64](crud, store,
		history.WithIDFunc[Adset, int64](func(a Adset) int64 { return a.ID }),
		history.WithRecordType[Adset, int64]("adsets"),
		history.WithParentRef[Adset, int64]("campaigns", "CampaignID"),
	)

	ctx := context.Background()
	adset, _ := repo.Create(ctx, Adset{CampaignID: 5, Name: "Test Adset"})

	records, _, _ := store.ForRecord(ctx, "adsets", strconv.FormatInt(adset.ID, 10))
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

	// Now test ForTree
	treeRecords, count, _ := store.ForTree(ctx, []history.TreeScope{
		{RecordType: "campaigns", RecordID: "5"},
		{RecordType: "adsets", ParentType: "campaigns", ParentID: "5"},
	})
	if count != 1 {
		t.Fatalf("expected 1 tree record, got %d: %v", count, treeRecords)
	}
}

func TestReverter_RevertTo(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()

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
	records, count, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
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
	store := newMemoryStore()

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
	store := newMemoryStore()

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
	store := newMemoryStore()

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
	store := newMemoryStore()

	// Don't set RecordType — should auto-derive from Order -> "orders"
	repo := history.WithHistory[Order, int64](crud, store,
		history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
	)

	ctx := context.Background()
	order, _ := repo.Create(ctx, Order{Name: "Test", Total: 1, Status: "ok"})

	records, _, _ := store.ForRecord(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].RecordType != "orders" {
		t.Errorf("expected RecordType 'orders', got %q", records[0].RecordType)
	}
}

func TestCRUD_SnapshotRules(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()
	snapStore := newMemorySnapshotStore()

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

	snaps, _ := snapStore.ListSnapshots(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots after create with draft status, got %d", len(snaps))
	}

	// Update status to "published" — should trigger snapshot
	order.Status = "published"
	order, _ = repo.Update(ctx, order)

	snaps, _ = snapStore.ListSnapshots(ctx, "orders", strconv.FormatInt(order.ID, 10))
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

	snaps, _ = snapStore.ListSnapshots(ctx, "orders", strconv.FormatInt(order.ID, 10))
	if len(snaps) != 1 {
		t.Fatalf("expected still 1 snapshot after non-publish update, got %d", len(snaps))
	}
}

func TestCRUD_SnapshotRules_MultipleRules(t *testing.T) {
	crud := newMemoryCRUD()
	store := newMemoryStore()
	snapStore1 := newMemorySnapshotStore()
	snapStore2 := newMemorySnapshotStore()

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
	snaps1, _ := snapStore1.ListSnapshots(ctx, "orders", strconv.FormatInt(order.ID, 10))
	snaps2, _ := snapStore2.ListSnapshots(ctx, "orders", strconv.FormatInt(order.ID, 10))

	if len(snaps1) != 0 {
		t.Errorf("expected 0 publish snapshots, got %d", len(snaps1))
	}
	if len(snaps2) != 1 {
		t.Errorf("expected 1 high-value snapshot, got %d", len(snaps2))
	}

	// Now publish — Rule 1 fires, Rule 2 does not (not a create)
	order.Status = "published"
	order, _ = repo.Update(ctx, order)

	snaps1, _ = snapStore1.ListSnapshots(ctx, "orders", strconv.FormatInt(order.ID, 10))
	snaps2, _ = snapStore2.ListSnapshots(ctx, "orders", strconv.FormatInt(order.ID, 10))

	if len(snaps1) != 1 {
		t.Errorf("expected 1 publish snapshot, got %d", len(snaps1))
	}
	if len(snaps2) != 1 {
		t.Errorf("expected still 1 high-value snapshot, got %d", len(snaps2))
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
