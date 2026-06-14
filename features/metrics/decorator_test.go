package metrics_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/metrics"
)

// ── Test Entity ──────────────────────────────────────────────────────────

type Order struct {
	ID        int64     `db:"id,pk"      json:"id"`
	Name      string    `db:"name"       json:"name"`
	Total     int       `db:"total"      json:"total"`
	Status    string    `db:"status"     json:"status"`
	Type      string    `db:"type"       json:"type"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ── In-Memory CRUD (mock for entity) ─────────────────────────────────────

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
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = time.Now()
	}
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
		case "type":
			existing.Type = entity.Type
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

// ── In-Memory CRUD for MetricRecord ──────────────────────────────────────

type memoryMetricsCRUD struct {
	mu   sync.Mutex
	data map[string]metrics.MetricRecord
}

func newMemoryMetricsCRUD() *memoryMetricsCRUD {
	return &memoryMetricsCRUD{data: make(map[string]metrics.MetricRecord)}
}

func (m *memoryMetricsCRUD) Create(_ context.Context, record metrics.MetricRecord) (metrics.MetricRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record.ID == "" {
		record.ID = fmt.Sprintf("m_%d", len(m.data)+1)
	}
	m.data[record.ID] = record
	return record, nil
}

func (m *memoryMetricsCRUD) Get(_ context.Context, id string, _ ...r3.Query) (metrics.MetricRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, ok := m.data[id]
	if !ok {
		return metrics.MetricRecord{}, fmt.Errorf("not found: %s", id)
	}
	return record, nil
}

func (m *memoryMetricsCRUD) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, qarg...)
	return n, err
}

func (m *memoryMetricsCRUD) List(_ context.Context, qarg ...r3.Query) ([]metrics.MetricRecord, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []metrics.MetricRecord
	for _, v := range m.data {
		if len(qarg) > 0 && len(qarg[0].Filters) > 0 {
			if !matchesFilters(v, qarg[0].Filters) {
				continue
			}
		}
		result = append(result, v)
	}

	// Sort by created_at desc (default)
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, int64(len(result)), nil
}

func (m *memoryMetricsCRUD) Update(_ context.Context, record metrics.MetricRecord) (metrics.MetricRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[record.ID] = record
	return record, nil
}

func (m *memoryMetricsCRUD) Patch(
	_ context.Context,
	record metrics.MetricRecord,
	_ r3.Fields,
) (metrics.MetricRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[record.ID] = record
	return record, nil
}

func (m *memoryMetricsCRUD) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
	return nil
}

// allRecords returns all stored metric records (for test assertions).
func (m *memoryMetricsCRUD) allRecords() []metrics.MetricRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]metrics.MetricRecord, 0, len(m.data))
	for _, v := range m.data {
		result = append(result, v)
	}
	return result
}

// recordsByMetric returns metric records filtered by metric name.
func (m *memoryMetricsCRUD) recordsByMetric(name string) []metrics.MetricRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []metrics.MetricRecord
	for _, v := range m.data {
		if v.MetricName == name {
			result = append(result, v)
		}
	}
	return result
}

// matchesFilters is a minimal filter matcher for our in-memory store.
// Only supports AND groups with Eq on record_type, metric_name, record_id, bucket.
func matchesFilters(record metrics.MetricRecord, filters r3.Filters) bool {
	for _, f := range filters {
		if f == nil {
			continue
		}
		// Handle AND groups
		if len(f.And) > 0 {
			for _, af := range f.And {
				if af.Field == nil {
					continue
				}
				fieldName := af.Field.String()
				switch fieldName {
				case "record_type":
					if record.RecordType != fmt.Sprint(af.Value) {
						return false
					}
				case "metric_name":
					if record.MetricName != fmt.Sprint(af.Value) {
						return false
					}
				case "record_id":
					if record.RecordID != fmt.Sprint(af.Value) {
						return false
					}
				case "bucket":
					if record.Bucket != fmt.Sprint(af.Value) {
						return false
					}
				case "created_at":
					// Handle Gte/Lte for time range
					if t, ok := af.Value.(time.Time); ok {
						if af.Operator == r3.OperatorGte && record.CreatedAt.Before(t) {
							return false
						}
						if af.Operator == r3.OperatorLte && record.CreatedAt.After(t) {
							return false
						}
					}
				}
			}
		}
		// Handle leaf filter
		if f.Field != nil {
			fieldName := f.Field.String()
			switch fieldName {
			case "record_type":
				if record.RecordType != fmt.Sprint(f.Value) {
					return false
				}
			case "metric_name":
				if record.MetricName != fmt.Sprint(f.Value) {
					return false
				}
			}
		}
	}
	return true
}

// ── Helpers ──────────────────────────────────────────────────────────────

func orderIDFunc(o Order) int64 { return o.ID }

func newTestRepo(
	inner *memoryCRUD,
	store *memoryMetricsCRUD,
	opts ...metrics.Option[Order, int64],
) *metrics.CRUD[Order, int64] {
	defaultOpts := []metrics.Option[Order, int64]{
		metrics.WithIDFunc[Order, int64](orderIDFunc),
	}
	return metrics.WithMetrics[Order, int64](inner, store, append(defaultOpts, opts...)...)
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestCRUD_CreateRecordsMetrics(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	order, err := repo.Create(ctx, Order{Name: "Test", Total: 100, Status: "new"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(records) != 1 {
		t.Fatalf("expected 1 metric record, got %d", len(records))
	}

	rec := records[0]
	if rec.RecordType != "orders" {
		t.Errorf("expected RecordType 'orders', got %q", rec.RecordType)
	}
	expectedID := strconv.FormatInt(order.ID, 10)
	if rec.RecordID != expectedID {
		t.Errorf("expected RecordID %q, got %q", expectedID, rec.RecordID)
	}
	if rec.Value != 1 {
		t.Errorf("expected Value 1, got %f", rec.Value)
	}

	// Check core labels
	labels := rec.Labels.Val
	if labels["operation"] != "create" {
		t.Errorf("expected operation label 'create', got %q", labels["operation"])
	}
	if labels["actor_type"] != "system" {
		t.Errorf("expected actor_type 'system', got %q", labels["actor_type"])
	}
}

func TestCRUD_GetRecordsMetrics(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
			metrics.PopularityCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	created, _ := repo.Create(ctx, Order{Name: "Test"})

	// Clear metrics from Create
	store.mu.Lock()
	store.data = make(map[string]metrics.MetricRecord)
	store.mu.Unlock()

	_, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(actionRecords) != 1 {
		t.Fatalf("expected 1 crud.action record, got %d", len(actionRecords))
	}
	if actionRecords[0].Labels.Val["operation"] != "get" {
		t.Errorf("expected operation 'get', got %q", actionRecords[0].Labels.Val["operation"])
	}

	popRecords := store.recordsByMetric(metrics.MetricEntityPopularity)
	if len(popRecords) != 1 {
		t.Fatalf("expected 1 popularity record, got %d", len(popRecords))
	}
	if popRecords[0].Labels.Val["source"] != "get" {
		t.Errorf("expected source 'get', got %q", popRecords[0].Labels.Val["source"])
	}
}

func TestCRUD_ListRecordsMetrics(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
			metrics.ListResultSizeCollector[Order, int64](),
			metrics.ListTotalCountCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "A"})
	repo.Create(ctx, Order{Name: "B"})

	// Clear metrics from Creates
	store.mu.Lock()
	store.data = make(map[string]metrics.MetricRecord)
	store.mu.Unlock()

	results, total, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(results) != 2 || total != 2 {
		t.Fatalf("expected 2 results, got %d/%d", len(results), total)
	}

	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(actionRecords) != 1 {
		t.Fatalf("expected 1 crud.action record, got %d", len(actionRecords))
	}
	if actionRecords[0].Labels.Val["operation"] != "list" {
		t.Errorf("expected operation 'list', got %q", actionRecords[0].Labels.Val["operation"])
	}

	sizeRecords := store.recordsByMetric(metrics.MetricListResultSize)
	if len(sizeRecords) != 1 {
		t.Fatalf("expected 1 result_size record, got %d", len(sizeRecords))
	}
	if sizeRecords[0].Value != 2 {
		t.Errorf("expected result_size value 2, got %f", sizeRecords[0].Value)
	}

	totalRecords := store.recordsByMetric(metrics.MetricListTotalCount)
	if len(totalRecords) != 1 {
		t.Fatalf("expected 1 total_count record, got %d", len(totalRecords))
	}
	if totalRecords[0].Value != 2 {
		t.Errorf("expected total_count value 2, got %f", totalRecords[0].Value)
	}
}

func TestCRUD_UpdateRecordsMetrics(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
			metrics.FieldChangeCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	created, _ := repo.Create(ctx, Order{Name: "Original", Total: 100, Status: "new"})

	// Clear metrics from Create
	store.mu.Lock()
	store.data = make(map[string]metrics.MetricRecord)
	store.mu.Unlock()

	created.Name = "Updated"
	created.Total = 200
	_, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(actionRecords) != 1 {
		t.Fatalf("expected 1 crud.action record, got %d", len(actionRecords))
	}
	if actionRecords[0].Labels.Val["operation"] != "update" {
		t.Errorf("expected operation 'update', got %q", actionRecords[0].Labels.Val["operation"])
	}

	changeRecords := store.recordsByMetric(metrics.MetricEntityFieldChange)
	if len(changeRecords) < 2 {
		t.Fatalf("expected at least 2 field_change records (Name+Total), got %d", len(changeRecords))
	}
}

func TestCRUD_DeleteRecordsMetrics(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	created, _ := repo.Create(ctx, Order{Name: "ToDelete"})

	// Clear metrics from Create
	store.mu.Lock()
	store.data = make(map[string]metrics.MetricRecord)
	store.mu.Unlock()

	err := repo.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(records) != 1 {
		t.Fatalf("expected 1 metric record, got %d", len(records))
	}
	if records[0].Labels.Val["operation"] != "delete" {
		t.Errorf("expected operation 'delete', got %q", records[0].Labels.Val["operation"])
	}
}

func TestCRUD_ActorContext(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "42", Type: "user"})
	_, err := repo.Create(ctx, Order{Name: "Test"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	labels := records[0].Labels.Val
	if labels["actor_id"] != "42" {
		t.Errorf("expected actor_id '42', got %q", labels["actor_id"])
	}
	if labels["actor_type"] != "user" {
		t.Errorf("expected actor_type 'user', got %q", labels["actor_type"])
	}
}

func TestCRUD_EntityLabelers(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
		metrics.WithLabelers[Order, int64](
			func(o Order) metrics.Labels {
				return metrics.Labels{"order_type": o.Type}
			},
			func(o Order) metrics.Labels {
				return metrics.Labels{"status": o.Status}
			},
		),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Order{Name: "Test", Type: "manual", Status: "new"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	labels := records[0].Labels.Val
	if labels["order_type"] != "manual" {
		t.Errorf("expected order_type 'manual', got %q", labels["order_type"])
	}
	if labels["status"] != "new" {
		t.Errorf("expected status 'new', got %q", labels["status"])
	}
}

func TestCRUD_ContextLabelers(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
		metrics.WithContextLabelers[Order, int64](
			func(_ context.Context) metrics.Labels {
				return metrics.Labels{"source": "api", "version": "v2"}
			},
		),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Order{Name: "Test"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	labels := records[0].Labels.Val
	if labels["source"] != "api" {
		t.Errorf("expected source 'api', got %q", labels["source"])
	}
	if labels["version"] != "v2" {
		t.Errorf("expected version 'v2', got %q", labels["version"])
	}
}

// TestCRUD_AsyncUsesDetachedContext verifies that in async mode collectors (and
// context labelers / actor extraction, which share the same context) observe the
// detached recording context, not the original request context after it has been
// cancelled (C4). With the bug, the collector saw context.Canceled.
func TestCRUD_AsyncUsesDetachedContext(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	observed := make(chan error, 1)

	// A collector that records the context's cancellation state, but only after
	// the test has cancelled the request context.
	gated := metrics.CollectorFunc[Order, int64](
		func(ctx context.Context, _ metrics.OperationContext[Order, int64]) []metrics.MetricEntry {
			close(started)
			<-release
			observed <- ctx.Err()
			return []metrics.MetricEntry{{MetricName: "test", Value: 1}}
		},
	)

	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](gated),
		metrics.WithAsync[Order, int64](),
	)

	ctx, cancel := context.WithCancel(r3.WithActor(context.Background(), r3.Actor{ID: "7", Type: "user"}))
	if _, err := repo.Create(ctx, Order{Name: "Test"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	<-started // the async goroutine has reached the collector
	cancel()  // cancel the request context, as a handler returning would
	close(release)

	select {
	case err := <-observed:
		if err != nil {
			t.Errorf("collector observed cancelled context %v; recording must use the detached context", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async recording")
	}

	// The detached context must still carry request-scoped values (the Actor).
	deadline := time.Now().Add(2 * time.Second)
	for {
		records := store.recordsByMetric("test")
		if len(records) == 1 {
			if got := records[0].Labels.Val["actor_id"]; got != "7" {
				t.Errorf("expected actor_id '7' preserved on detached context, got %q", got)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCRUD_LatencyCollector(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.LatencyCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Order{Name: "Test"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	records := store.recordsByMetric(metrics.MetricCRUDLatency)
	if len(records) != 1 {
		t.Fatalf("expected 1 latency record, got %d", len(records))
	}
	if records[0].Value < 0 {
		t.Errorf("expected non-negative latency, got %f", records[0].Value)
	}
}

func TestCRUD_ErrorCollector(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.ErrorCollector[Order, int64](),
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()

	// Get a non-existent entity to trigger an error
	_, err := repo.Get(ctx, 999)
	if err == nil {
		t.Fatal("expected error from Get of non-existent ID")
	}

	errorRecords := store.recordsByMetric(metrics.MetricCRUDError)
	if len(errorRecords) != 1 {
		t.Fatalf("expected 1 error record, got %d", len(errorRecords))
	}

	// CRUDActionCollector should NOT fire on error
	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	if len(actionRecords) != 0 {
		t.Errorf("expected 0 crud.action records on error, got %d", len(actionRecords))
	}
}

func TestCRUD_PatchSizeCollector(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.PatchSizeCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	created, _ := repo.Create(ctx, Order{Name: "Test", Status: "new"})

	// Clear metrics from Create
	store.mu.Lock()
	store.data = make(map[string]metrics.MetricRecord)
	store.mu.Unlock()

	_, err := repo.Patch(ctx, Order{ID: created.ID, Name: "Updated", Status: "done"}, r3.Fields{
		r3.NewFieldSpec("name"),
		r3.NewFieldSpec("status"),
	})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}

	records := store.recordsByMetric(metrics.MetricEntityPatchSize)
	if len(records) != 1 {
		t.Fatalf("expected 1 patch_size record, got %d", len(records))
	}
	if records[0].Value != 2 {
		t.Errorf("expected patch_size 2, got %f", records[0].Value)
	}
}

func TestCRUD_ListFilterCollector(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.ListFilterCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "A", Status: "active"})

	// Clear metrics from Create
	store.mu.Lock()
	store.data = make(map[string]metrics.MetricRecord)
	store.mu.Unlock()

	q := r3.Query{
		Filters: r3.Filters{
			r3.F(r3.NewFieldSpec("status"), "active"),
			r3.Fop(r3.NewFieldSpec("total"), r3.OperatorGt, 50),
		},
	}
	_, _, err := repo.List(ctx, q)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	filterRecords := store.recordsByMetric(metrics.MetricListFilter)
	if len(filterRecords) != 2 {
		t.Fatalf("expected 2 filter records, got %d", len(filterRecords))
	}

	// Check that we captured the filter fields
	fields := make(map[string]bool)
	for _, r := range filterRecords {
		fields[r.Labels.Val["field"]] = true
	}
	if !fields["status"] {
		t.Error("expected filter record for 'status'")
	}
	if !fields["total"] {
		t.Error("expected filter record for 'total'")
	}
}

func TestCRUD_DeriveRecordType(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "Test"})

	records := store.allRecords()
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	if records[0].RecordType != "orders" {
		t.Errorf("expected auto-derived RecordType 'orders', got %q", records[0].RecordType)
	}
}

func TestCRUD_CustomRecordType(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithRecordType[Order, int64]("custom_orders"),
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "Test"})

	records := store.allRecords()
	if records[0].RecordType != "custom_orders" {
		t.Errorf("expected RecordType 'custom_orders', got %q", records[0].RecordType)
	}
}

func TestCRUD_BucketSize(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithBucketSize[Order, int64](metrics.BucketHourly),
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "Test"})

	records := store.allRecords()
	if len(records) == 0 {
		t.Fatal("expected at least 1 record")
	}
	// Hourly bucket should have minutes and seconds as 00
	bucket := records[0].Bucket
	parsed, err := time.Parse(time.RFC3339, bucket)
	if err != nil {
		t.Fatalf("failed to parse bucket: %v", err)
	}
	if parsed.Minute() != 0 || parsed.Second() != 0 {
		t.Errorf("expected hourly bucket with 0 minutes/seconds, got %s", bucket)
	}
}

func TestCRUD_NoCollectorsNoRecords(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store) // no collectors

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "Test"})

	records := store.allRecords()
	if len(records) != 0 {
		t.Errorf("expected 0 records with no collectors, got %d", len(records))
	}
}

func TestCRUD_InnerAccess(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	// Inner() should bypass metrics
	ctx := context.Background()
	repo.Inner().Create(ctx, Order{Name: "Bypass"})

	records := store.allRecords()
	if len(records) != 0 {
		t.Errorf("expected 0 records via Inner(), got %d", len(records))
	}
}

func TestCRUD_MetricsStoreAccess(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store)

	metricsStore := repo.Metrics()
	if metricsStore == nil {
		t.Fatal("expected non-nil Metrics() store")
	}
}

func TestCRUD_MultipleCollectors(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
			metrics.LatencyCollector[Order, int64](),
			metrics.PopularityCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	created, _ := repo.Create(ctx, Order{Name: "Test"})

	// Clear create metrics
	store.mu.Lock()
	store.data = make(map[string]metrics.MetricRecord)
	store.mu.Unlock()

	// Get triggers CRUDAction + Latency + Popularity
	repo.Get(ctx, created.ID)

	all := store.allRecords()
	if len(all) != 3 {
		t.Fatalf("expected 3 records (action+latency+popularity), got %d", len(all))
	}

	metricNames := make(map[string]bool)
	for _, r := range all {
		metricNames[r.MetricName] = true
	}
	if !metricNames[metrics.MetricCRUDAction] {
		t.Error("expected crud.action metric")
	}
	if !metricNames[metrics.MetricCRUDLatency] {
		t.Error("expected crud.action.latency metric")
	}
	if !metricNames[metrics.MetricEntityPopularity] {
		t.Error("expected entity.popularity metric")
	}
}

func TestCRUD_CustomCollector(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()

	customCollector := metrics.CollectorFunc[Order, int64](
		func(_ context.Context, opCtx metrics.OperationContext[Order, int64]) []metrics.MetricEntry {
			if opCtx.Err != nil || opCtx.Operation != metrics.OpCreate {
				return nil
			}
			return []metrics.MetricEntry{{
				MetricName: "custom.order_value",
				Value:      float64(opCtx.Entity.Total),
				Labels:     metrics.Labels{"custom": "yes"},
			}}
		},
	)

	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](customCollector),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "Expensive", Total: 999})

	records := store.recordsByMetric("custom.order_value")
	if len(records) != 1 {
		t.Fatalf("expected 1 custom record, got %d", len(records))
	}
	if records[0].Value != 999 {
		t.Errorf("expected value 999, got %f", records[0].Value)
	}
	if records[0].Labels.Val["custom"] != "yes" {
		t.Errorf("expected custom label 'yes', got %q", records[0].Labels.Val["custom"])
	}
	// Core labels should also be present
	if records[0].Labels.Val["operation"] != "create" {
		t.Errorf("expected operation label 'create', got %q", records[0].Labels.Val["operation"])
	}
}

// ── Aggregator Tests ─────────────────────────────────────────────────────

func TestAggregator_Count(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "A"})
	repo.Create(ctx, Order{Name: "B"})
	repo.Create(ctx, Order{Name: "C"})

	agg := metrics.NewAggregator(store)
	count, err := agg.Count(ctx, "orders", metrics.MetricCRUDAction, metrics.LastDays(1),
		metrics.WithLabel("operation", "create"),
	)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestAggregator_Sum(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.LatencyCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "A"})
	repo.Create(ctx, Order{Name: "B"})

	agg := metrics.NewAggregator(store)
	sum, err := agg.Sum(ctx, "orders", metrics.MetricCRUDLatency, metrics.LastDays(1))
	if err != nil {
		t.Fatalf("Sum failed: %v", err)
	}
	if sum < 0 {
		t.Errorf("expected non-negative sum, got %f", sum)
	}
}

func TestAggregator_GroupBy(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	created, _ := repo.Create(ctx, Order{Name: "A"})
	repo.Get(ctx, created.ID)
	repo.Get(ctx, created.ID)

	agg := metrics.NewAggregator(store)
	groups, err := agg.GroupBy(ctx, "orders", metrics.MetricCRUDAction, metrics.LastDays(1), "operation")
	if err != nil {
		t.Fatalf("GroupBy failed: %v", err)
	}

	if groups["create"] != 1 {
		t.Errorf("expected 1 create, got %f", groups["create"])
	}
	if groups["get"] != 2 {
		t.Errorf("expected 2 gets, got %f", groups["get"])
	}
}

// ── AggregationPusher Tests ──────────────────────────────────────────────

// mockPusherStore implements both r3.CRUD[MetricRecord, string] and AggregationPusher
// to test server-side aggregation delegation.
type mockPusherStore struct {
	*memoryMetricsCRUD
	pushCountCalls      int
	pushSumCalls        int
	pushAvgCalls        int
	pushTopNCalls       int
	pushTimeSeriesCalls int
	pushGroupByCalls    int
	lastLabels          metrics.Labels
}

func newMockPusherStore() *mockPusherStore {
	return &mockPusherStore{memoryMetricsCRUD: newMemoryMetricsCRUD()}
}

func (m *mockPusherStore) PushCount(
	_ context.Context,
	_, _ string,
	_ metrics.TimeRange,
	labels metrics.Labels,
) (int64, error) {
	m.pushCountCalls++
	m.lastLabels = labels
	return 42, nil
}

func (m *mockPusherStore) PushSum(
	_ context.Context,
	_, _ string,
	_ metrics.TimeRange,
	labels metrics.Labels,
) (float64, error) {
	m.pushSumCalls++
	m.lastLabels = labels
	return 123.45, nil
}

func (m *mockPusherStore) PushAvg(
	_ context.Context,
	_, _ string,
	_ metrics.TimeRange,
	labels metrics.Labels,
) (float64, error) {
	m.pushAvgCalls++
	m.lastLabels = labels
	return 41.15, nil
}

func (m *mockPusherStore) PushTopN(
	_ context.Context,
	_, _ string,
	_ metrics.TimeRange,
	n int,
	labels metrics.Labels,
) ([]metrics.RankedEntity, error) {
	m.pushTopNCalls++
	m.lastLabels = labels
	result := make([]metrics.RankedEntity, 0, n)
	for i := range n {
		result = append(result, metrics.RankedEntity{
			RecordID: fmt.Sprintf("entity_%d", i+1),
			Value:    float64(100 - i*10),
		})
	}
	return result, nil
}

func (m *mockPusherStore) PushTimeSeries(
	_ context.Context,
	_, _ string,
	_ metrics.TimeRange,
	_ metrics.BucketSize,
	labels metrics.Labels,
) ([]metrics.BucketValue, error) {
	m.pushTimeSeriesCalls++
	m.lastLabels = labels
	return []metrics.BucketValue{
		{Bucket: "2026-02-16T00:00:00Z", Value: 10},
		{Bucket: "2026-02-17T00:00:00Z", Value: 20},
	}, nil
}

func (m *mockPusherStore) PushGroupBy(
	_ context.Context,
	_, _ string,
	_ metrics.TimeRange,
	_ string,
	labels metrics.Labels,
) (map[string]float64, error) {
	m.pushGroupByCalls++
	m.lastLabels = labels
	return map[string]float64{"create": 10, "get": 20}, nil
}

func TestAggregator_PusherDelegation_Count(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	count, err := agg.Count(ctx, "orders", "crud.action", metrics.LastDays(1),
		metrics.WithLabel("operation", "create"),
	)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 42 {
		t.Errorf("expected count 42 from pusher, got %d", count)
	}
	if store.pushCountCalls != 1 {
		t.Errorf("expected 1 PushCount call, got %d", store.pushCountCalls)
	}
	if store.lastLabels["operation"] != "create" {
		t.Errorf("expected label operation=create, got %v", store.lastLabels)
	}
}

func TestAggregator_PusherDelegation_Sum(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	sum, err := agg.Sum(ctx, "orders", "crud.action.latency", metrics.LastDays(1))
	if err != nil {
		t.Fatalf("Sum failed: %v", err)
	}
	if sum != 123.45 {
		t.Errorf("expected sum 123.45 from pusher, got %f", sum)
	}
	if store.pushSumCalls != 1 {
		t.Errorf("expected 1 PushSum call, got %d", store.pushSumCalls)
	}
}

func TestAggregator_PusherDelegation_Avg(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	avg, err := agg.Avg(ctx, "orders", "crud.action.latency", metrics.LastDays(1))
	if err != nil {
		t.Fatalf("Avg failed: %v", err)
	}
	if avg != 41.15 {
		t.Errorf("expected avg 41.15 from pusher, got %f", avg)
	}
	if store.pushAvgCalls != 1 {
		t.Errorf("expected 1 PushAvg call, got %d", store.pushAvgCalls)
	}
}

func TestAggregator_PusherDelegation_TopN(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	top, err := agg.TopN(ctx, "orders", "entity.popularity", metrics.LastDays(1), 3)
	if err != nil {
		t.Fatalf("TopN failed: %v", err)
	}
	if len(top) != 3 {
		t.Fatalf("expected 3 results from pusher, got %d", len(top))
	}
	if top[0].RecordID != "entity_1" {
		t.Errorf("expected first entity 'entity_1', got %q", top[0].RecordID)
	}
	if store.pushTopNCalls != 1 {
		t.Errorf("expected 1 PushTopN call, got %d", store.pushTopNCalls)
	}
}

func TestAggregator_PusherDelegation_TimeSeries(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	ts, err := agg.TimeSeries(ctx, "orders", "crud.action", metrics.LastDays(1), metrics.BucketDaily)
	if err != nil {
		t.Fatalf("TimeSeries failed: %v", err)
	}
	if len(ts) != 2 {
		t.Fatalf("expected 2 buckets from pusher, got %d", len(ts))
	}
	if ts[0].Value != 10 || ts[1].Value != 20 {
		t.Errorf("unexpected bucket values: %v", ts)
	}
	if store.pushTimeSeriesCalls != 1 {
		t.Errorf("expected 1 PushTimeSeries call, got %d", store.pushTimeSeriesCalls)
	}
}

func TestAggregator_PusherDelegation_GroupBy(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	groups, err := agg.GroupBy(ctx, "orders", "crud.action", metrics.LastDays(1), "operation")
	if err != nil {
		t.Fatalf("GroupBy failed: %v", err)
	}
	if groups["create"] != 10 {
		t.Errorf("expected create=10, got %f", groups["create"])
	}
	if groups["get"] != 20 {
		t.Errorf("expected get=20, got %f", groups["get"])
	}
	if store.pushGroupByCalls != 1 {
		t.Errorf("expected 1 PushGroupBy call, got %d", store.pushGroupByCalls)
	}
}

func TestAggregator_FallbackToInMemory(t *testing.T) {
	// Regular store (not implementing AggregationPusher) should use in-memory aggregation.
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store,
		metrics.WithCollectors[Order, int64](
			metrics.CRUDActionCollector[Order, int64](),
		),
	)

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "A"})
	repo.Create(ctx, Order{Name: "B"})

	agg := metrics.NewAggregator(store)
	count, err := agg.Count(ctx, "orders", metrics.MetricCRUDAction, metrics.LastDays(1),
		metrics.WithLabel("operation", "create"),
	)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2 from in-memory, got %d", count)
	}
}

// ── Bucket Tests ─────────────────────────────────────────────────────────

func TestComputeBucket(t *testing.T) {
	ts := time.Date(2026, 2, 16, 14, 32, 45, 0, time.UTC)

	tests := []struct {
		size     metrics.BucketSize
		expected string
	}{
		{metrics.BucketMinutely, "2026-02-16T14:32:00Z"},
		{metrics.BucketHourly, "2026-02-16T14:00:00Z"},
		{metrics.BucketDaily, "2026-02-16T00:00:00Z"},
		{metrics.BucketMonthly, "2026-02-01T00:00:00Z"},
	}

	for _, tt := range tests {
		result := metrics.ComputeBucket(ts, tt.size)
		if result != tt.expected {
			t.Errorf("ComputeBucket(%s) = %q, want %q", tt.size, result, tt.expected)
		}
	}
}

func TestComputeBucket_Weekly(t *testing.T) {
	// 2026-02-16 is a Monday
	monday := time.Date(2026, 2, 16, 14, 0, 0, 0, time.UTC)
	result := metrics.ComputeBucket(monday, metrics.BucketWeekly)
	if result != "2026-02-16T00:00:00Z" {
		t.Errorf("Monday bucket = %q, want 2026-02-16T00:00:00Z", result)
	}

	// Wednesday should still be in the same week
	wednesday := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)
	result = metrics.ComputeBucket(wednesday, metrics.BucketWeekly)
	if result != "2026-02-16T00:00:00Z" {
		t.Errorf("Wednesday bucket = %q, want 2026-02-16T00:00:00Z", result)
	}
}

// ── Labels Tests ─────────────────────────────────────────────────────────

func TestLabels_Merge(t *testing.T) {
	a := metrics.Labels{"a": "1", "b": "2"}
	b := metrics.Labels{"b": "override", "c": "3"}

	merged := a.Merge(b)
	if merged["a"] != "1" {
		t.Errorf("expected a=1, got %q", merged["a"])
	}
	if merged["b"] != "override" {
		t.Errorf("expected b=override, got %q", merged["b"])
	}
	if merged["c"] != "3" {
		t.Errorf("expected c=3, got %q", merged["c"])
	}
}

func TestLabels_MergeNil(t *testing.T) {
	var a metrics.Labels
	b := metrics.Labels{"x": "1"}

	merged := a.Merge(b)
	if merged["x"] != "1" {
		t.Errorf("expected x=1, got %q", merged["x"])
	}

	merged2 := b.Merge(a)
	if merged2["x"] != "1" {
		t.Errorf("expected x=1, got %q", merged2["x"])
	}
}

// ── Retention Tests ──────────────────────────────────────────────────────

func TestRetentionEnforcer_MaxAge(t *testing.T) {
	store := newMemoryMetricsCRUD()

	// Insert records: some old, some new.
	now := time.Now().UTC()
	oldTime := now.Add(-48 * time.Hour) // 2 days ago
	newTime := now.Add(-1 * time.Hour)  // 1 hour ago

	store.Create(context.Background(), metrics.MetricRecord{
		ID: "old1", RecordType: "orders", MetricName: "crud.action",
		Value: 1, CreatedAt: oldTime,
		Labels: r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
	})
	store.Create(context.Background(), metrics.MetricRecord{
		ID: "old2", RecordType: "orders", MetricName: "crud.action",
		Value: 1, CreatedAt: oldTime.Add(time.Minute),
		Labels: r3.NewJSONColumn(metrics.Labels{"operation": "get"}),
	})
	store.Create(context.Background(), metrics.MetricRecord{
		ID: "new1", RecordType: "orders", MetricName: "crud.action",
		Value: 1, CreatedAt: newTime,
		Labels: r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
	})

	enforcer := metrics.NewRetentionEnforcer(store, metrics.RetentionPolicy{
		MaxAge: 24 * time.Hour, // delete records older than 24 hours
	})

	deleted := enforcer.Enforce(context.Background(), "orders")
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	remaining := store.allRecords()
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining record, got %d", len(remaining))
	}
	if remaining[0].ID != "new1" {
		t.Errorf("expected remaining record 'new1', got %q", remaining[0].ID)
	}
}

func TestRetentionEnforcer_MaxRecords(t *testing.T) {
	store := newMemoryMetricsCRUD()

	now := time.Now().UTC()
	// Insert 5 records with slightly different times.
	for i := range 5 {
		store.Create(context.Background(), metrics.MetricRecord{
			ID:         fmt.Sprintf("r%d", i+1),
			RecordType: "orders",
			MetricName: "crud.action",
			Value:      1,
			CreatedAt:  now.Add(-time.Duration(5-i) * time.Hour), // r1 is oldest, r5 is newest
			Labels:     r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
		})
	}

	enforcer := metrics.NewRetentionEnforcer(store, metrics.RetentionPolicy{
		MaxRecords: 3,
	})

	deleted := enforcer.Enforce(context.Background(), "orders")
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	remaining := store.allRecords()
	if len(remaining) != 3 {
		t.Fatalf("expected 3 remaining records, got %d", len(remaining))
	}
}

func TestRetentionEnforcer_NoOp(t *testing.T) {
	store := newMemoryMetricsCRUD()
	now := time.Now().UTC()
	store.Create(context.Background(), metrics.MetricRecord{
		ID: "r1", RecordType: "orders", MetricName: "crud.action",
		Value: 1, CreatedAt: now,
		Labels: r3.NewJSONColumn(metrics.Labels{}),
	})

	enforcer := metrics.NewRetentionEnforcer(store, metrics.RetentionPolicy{
		MaxAge:     24 * time.Hour,
		MaxRecords: 100,
	})

	deleted := enforcer.Enforce(context.Background(), "orders")
	if deleted != 0 {
		t.Errorf("expected 0 deleted (all within limits), got %d", deleted)
	}
}

// ── Rollup Tests ─────────────────────────────────────────────────────────

func TestRollupExecutor_BasicRollup(t *testing.T) {
	store := newMemoryMetricsCRUD()

	// Create 3 minutely records in the same hour.
	baseTime := time.Date(2026, 2, 15, 14, 0, 0, 0, time.UTC) // old enough
	for i := range 3 {
		store.Create(context.Background(), metrics.MetricRecord{
			ID:         fmt.Sprintf("m%d", i+1),
			RecordType: "orders",
			MetricName: "crud.action",
			Value:      1,
			Labels:     r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
			Bucket:     metrics.ComputeBucket(baseTime.Add(time.Duration(i)*time.Minute), metrics.BucketMinutely),
			CreatedAt:  baseTime.Add(time.Duration(i) * time.Minute),
		})
	}

	executor := metrics.NewRollupExecutor(store, metrics.RollupPolicy{
		SourceBucket: metrics.BucketMinutely,
		TargetBucket: metrics.BucketHourly,
		MinAge:       1 * time.Hour,
	})

	deleted, created := executor.Execute(context.Background(), "orders")
	if deleted != 3 {
		t.Errorf("expected 3 deleted, got %d", deleted)
	}
	if created != 1 {
		t.Errorf("expected 1 summary created, got %d", created)
	}

	remaining := store.allRecords()
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining record, got %d", len(remaining))
	}

	summary := remaining[0]
	if summary.Value != 3 {
		t.Errorf("expected summed value 3, got %f", summary.Value)
	}
	if summary.Labels.Val["_rollup"] != "true" {
		t.Errorf("expected _rollup label, got %v", summary.Labels.Val)
	}
	if summary.Labels.Val["_rollup_count"] != "3" {
		t.Errorf("expected _rollup_count=3, got %q", summary.Labels.Val["_rollup_count"])
	}
	expectedBucket := metrics.ComputeBucket(baseTime, metrics.BucketHourly)
	if summary.Bucket != expectedBucket {
		t.Errorf("expected bucket %q, got %q", expectedBucket, summary.Bucket)
	}
}

func TestRollupExecutor_PreservesRecentRecords(t *testing.T) {
	store := newMemoryMetricsCRUD()

	// Mix of old and new records.
	oldTime := time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC)
	newTime := time.Now().UTC()

	store.Create(context.Background(), metrics.MetricRecord{
		ID: "old1", RecordType: "orders", MetricName: "crud.action",
		Value: 1, CreatedAt: oldTime,
		Labels: r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
		Bucket: metrics.ComputeBucket(oldTime, metrics.BucketMinutely),
	})
	store.Create(context.Background(), metrics.MetricRecord{
		ID: "new1", RecordType: "orders", MetricName: "crud.action",
		Value: 1, CreatedAt: newTime,
		Labels: r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
		Bucket: metrics.ComputeBucket(newTime, metrics.BucketMinutely),
	})

	executor := metrics.NewRollupExecutor(store, metrics.RollupPolicy{
		SourceBucket: metrics.BucketMinutely,
		TargetBucket: metrics.BucketHourly,
		MinAge:       24 * time.Hour,
	})

	deleted, created := executor.Execute(context.Background(), "orders")
	if deleted != 1 {
		t.Errorf("expected 1 deleted (old only), got %d", deleted)
	}
	if created != 1 {
		t.Errorf("expected 1 summary created, got %d", created)
	}

	remaining := store.allRecords()
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining (1 new + 1 summary), got %d", len(remaining))
	}
}

func TestRollupExecutor_GroupsByMetricAndEntity(t *testing.T) {
	store := newMemoryMetricsCRUD()
	baseTime := time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC)

	// Two different metrics, same entity.
	store.Create(context.Background(), metrics.MetricRecord{
		ID: "a1", RecordType: "orders", RecordID: "1", MetricName: "crud.action",
		Value: 1, CreatedAt: baseTime,
		Labels: r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
	})
	store.Create(context.Background(), metrics.MetricRecord{
		ID: "a2", RecordType: "orders", RecordID: "1", MetricName: "crud.action.latency",
		Value: 50, CreatedAt: baseTime.Add(time.Minute),
		Labels: r3.NewJSONColumn(metrics.Labels{"operation": "create"}),
	})

	executor := metrics.NewRollupExecutor(store, metrics.RollupPolicy{
		SourceBucket: metrics.BucketMinutely,
		TargetBucket: metrics.BucketDaily,
		MinAge:       1 * time.Hour,
	})

	deleted, created := executor.Execute(context.Background(), "orders")
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}
	if created != 2 {
		t.Errorf("expected 2 summaries (one per metric), got %d", created)
	}
}

// ── Actor Tests ──────────────────────────────────────────────────────────

func TestActor_ContextRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Default is SystemActor
	actor := r3.GetActor(ctx)
	if actor.Type != "system" {
		t.Errorf("expected default actor type 'system', got %q", actor.Type)
	}

	// Set actor
	ctx = r3.WithActor(ctx, r3.Actor{ID: "99", Type: "admin"})
	actor = r3.GetActor(ctx)
	if actor.ID != "99" || actor.Type != "admin" {
		t.Errorf("expected actor {99, admin}, got {%s, %s}", actor.ID, actor.Type)
	}
}
