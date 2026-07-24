package metrics_test

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/expectto/be"

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
	be.NoError(t, err)

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	be.RequireThat(t, records, be.HaveLength(1))

	rec := records[0]
	be.AssertThat(t, rec.RecordType, be.Eq("orders"))
	expectedID := strconv.FormatInt(order.ID, 10)
	be.AssertThat(t, rec.RecordID, be.Eq(expectedID))
	be.AssertThat(t, rec.Value, be.Eq(float64(1)))

	// Check core labels
	labels := rec.Labels.Val
	be.AssertThat(t, labels["operation"], be.Eq("create"))
	be.AssertThat(t, labels["actor_type"], be.Eq("system"))
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
	be.NoError(t, err)

	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	be.RequireThat(t, actionRecords, be.HaveLength(1))
	be.AssertThat(t, actionRecords[0].Labels.Val["operation"], be.Eq("get"))

	popRecords := store.recordsByMetric(metrics.MetricEntityPopularity)
	be.RequireThat(t, popRecords, be.HaveLength(1))
	be.AssertThat(t, popRecords[0].Labels.Val["source"], be.Eq("get"))
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
	be.NoError(t, err)
	be.RequireThat(t, len(results) == 2 && total == 2, be.True())

	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	be.RequireThat(t, actionRecords, be.HaveLength(1))
	be.AssertThat(t, actionRecords[0].Labels.Val["operation"], be.Eq("list"))

	sizeRecords := store.recordsByMetric(metrics.MetricListResultSize)
	be.RequireThat(t, sizeRecords, be.HaveLength(1))
	be.AssertThat(t, sizeRecords[0].Value, be.Eq(float64(2)))

	totalRecords := store.recordsByMetric(metrics.MetricListTotalCount)
	be.RequireThat(t, totalRecords, be.HaveLength(1))
	be.AssertThat(t, totalRecords[0].Value, be.Eq(float64(2)))
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
	be.NoError(t, err)

	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	be.RequireThat(t, actionRecords, be.HaveLength(1))
	be.AssertThat(t, actionRecords[0].Labels.Val["operation"], be.Eq("update"))

	changeRecords := store.recordsByMetric(metrics.MetricEntityFieldChange)
	be.RequireThat(t, changeRecords, be.HaveLength(be.Gte(2)))
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
	be.NoError(t, err)

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	be.RequireThat(t, records, be.HaveLength(1))
	be.AssertThat(t, records[0].Labels.Val["operation"], be.Eq("delete"))
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
	be.NoError(t, err)

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	be.RequireThat(t, records, be.HaveLength(1))

	labels := records[0].Labels.Val
	be.AssertThat(t, labels["actor_id"], be.Eq("42"))
	be.AssertThat(t, labels["actor_type"], be.Eq("user"))
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
	be.NoError(t, err)

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	be.RequireThat(t, records, be.HaveLength(1))

	labels := records[0].Labels.Val
	be.AssertThat(t, labels["order_type"], be.Eq("manual"))
	be.AssertThat(t, labels["status"], be.Eq("new"))
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
	be.NoError(t, err)

	records := store.recordsByMetric(metrics.MetricCRUDAction)
	labels := records[0].Labels.Val
	be.AssertThat(t, labels["source"], be.Eq("api"))
	be.AssertThat(t, labels["version"], be.Eq("v2"))
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
	_, err := repo.Create(ctx, Order{Name: "Test"})
	be.NoError(t, err)

	<-started // the async goroutine has reached the collector
	cancel()  // cancel the request context, as a handler returning would
	close(release)

	var observedErr error
	be.Eventually(t, func() bool {
		select {
		case observedErr = <-observed:
			return true
		default:
			return false
		}
	}, be.True(), be.WithTimeout(2*time.Second), be.WithPolling(time.Millisecond))
	be.AssertThat(t, observedErr, be.Succeed())

	// The detached context must still carry request-scoped values (the Actor).
	be.Eventually(t, func() []metrics.MetricRecord {
		return store.recordsByMetric("test")
	}, be.HaveLength(1), be.WithTimeout(2*time.Second), be.WithPolling(time.Millisecond))

	records := store.recordsByMetric("test")
	be.AssertThat(t, records[0].Labels.Val["actor_id"], be.Eq("7"))
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
	be.NoError(t, err)

	records := store.recordsByMetric(metrics.MetricCRUDLatency)
	be.RequireThat(t, records, be.HaveLength(1))
	be.AssertThat(t, records[0].Value, be.Gte(0))
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
	be.Error(t, err)

	errorRecords := store.recordsByMetric(metrics.MetricCRUDError)
	be.RequireThat(t, errorRecords, be.HaveLength(1))

	// CRUDActionCollector should NOT fire on error
	actionRecords := store.recordsByMetric(metrics.MetricCRUDAction)
	be.AssertThat(t, actionRecords, be.Empty())
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
	be.NoError(t, err)

	records := store.recordsByMetric(metrics.MetricEntityPatchSize)
	be.RequireThat(t, records, be.HaveLength(1))
	be.AssertThat(t, records[0].Value, be.Eq(float64(2)))
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
	be.NoError(t, err)

	filterRecords := store.recordsByMetric(metrics.MetricListFilter)
	be.RequireThat(t, filterRecords, be.HaveLength(2))

	// Check that we captured the filter fields
	fields := make(map[string]bool)
	for _, r := range filterRecords {
		fields[r.Labels.Val["field"]] = true
	}
	be.AssertThat(t, fields["status"], be.True())
	be.AssertThat(t, fields["total"], be.True())
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
	be.RequireThat(t, records, be.NotEmpty())
	be.AssertThat(t, records[0].RecordType, be.Eq("orders"))
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
	be.AssertThat(t, records[0].RecordType, be.Eq("custom_orders"))
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
	be.RequireThat(t, records, be.NotEmpty())
	// Hourly bucket should have minutes and seconds as 00
	bucket := records[0].Bucket
	parsed, err := time.Parse(time.RFC3339, bucket)
	be.NoError(t, err)
	be.AssertThat(t, parsed.Minute() == 0 && parsed.Second() == 0, be.True())
}

func TestCRUD_NoCollectorsNoRecords(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store) // no collectors

	ctx := context.Background()
	repo.Create(ctx, Order{Name: "Test"})

	records := store.allRecords()
	be.AssertThat(t, records, be.Empty())
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
	be.AssertThat(t, records, be.Empty())
}

func TestCRUD_MetricsStoreAccess(t *testing.T) {
	inner := newMemoryCRUD()
	store := newMemoryMetricsCRUD()
	repo := newTestRepo(inner, store)

	metricsStore := repo.Metrics()
	be.RequireThat(t, metricsStore, be.NotNil())
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
	be.RequireThat(t, all, be.HaveLength(3))

	metricNames := make(map[string]bool)
	for _, r := range all {
		metricNames[r.MetricName] = true
	}
	be.AssertThat(t, metricNames[metrics.MetricCRUDAction], be.True())
	be.AssertThat(t, metricNames[metrics.MetricCRUDLatency], be.True())
	be.AssertThat(t, metricNames[metrics.MetricEntityPopularity], be.True())
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
	be.RequireThat(t, records, be.HaveLength(1))
	be.AssertThat(t, records[0].Value, be.Eq(float64(999)))
	be.AssertThat(t, records[0].Labels.Val["custom"], be.Eq("yes"))
	// Core labels should also be present
	be.AssertThat(t, records[0].Labels.Val["operation"], be.Eq("create"))
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
	be.NoError(t, err)
	be.AssertThat(t, count, be.Eq(int64(3)))
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
	be.NoError(t, err)
	be.AssertThat(t, sum, be.Gte(0))
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
	be.NoError(t, err)

	be.AssertThat(t, groups["create"], be.Eq(float64(1)))
	be.AssertThat(t, groups["get"], be.Eq(float64(2)))
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
	be.NoError(t, err)
	be.AssertThat(t, count, be.Eq(int64(42)))
	be.AssertThat(t, store.pushCountCalls, be.Eq(1))
	be.AssertThat(t, store.lastLabels["operation"], be.Eq("create"))
}

func TestAggregator_PusherDelegation_Sum(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	sum, err := agg.Sum(ctx, "orders", "crud.action.latency", metrics.LastDays(1))
	be.NoError(t, err)
	be.AssertThat(t, sum, be.Eq(123.45))
	be.AssertThat(t, store.pushSumCalls, be.Eq(1))
}

func TestAggregator_PusherDelegation_Avg(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	avg, err := agg.Avg(ctx, "orders", "crud.action.latency", metrics.LastDays(1))
	be.NoError(t, err)
	be.AssertThat(t, avg, be.Eq(41.15))
	be.AssertThat(t, store.pushAvgCalls, be.Eq(1))
}

func TestAggregator_PusherDelegation_TopN(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	top, err := agg.TopN(ctx, "orders", "entity.popularity", metrics.LastDays(1), 3)
	be.NoError(t, err)
	be.RequireThat(t, top, be.HaveLength(3))
	be.AssertThat(t, top[0].RecordID, be.Eq("entity_1"))
	be.AssertThat(t, store.pushTopNCalls, be.Eq(1))
}

func TestAggregator_PusherDelegation_TimeSeries(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	ts, err := agg.TimeSeries(ctx, "orders", "crud.action", metrics.LastDays(1), metrics.BucketDaily)
	be.NoError(t, err)
	be.RequireThat(t, ts, be.HaveLength(2))
	be.AssertThat(t, ts[0].Value == 10 && ts[1].Value == 20, be.True())
	be.AssertThat(t, store.pushTimeSeriesCalls, be.Eq(1))
}

func TestAggregator_PusherDelegation_GroupBy(t *testing.T) {
	store := newMockPusherStore()
	agg := metrics.NewAggregator(store)

	ctx := context.Background()
	groups, err := agg.GroupBy(ctx, "orders", "crud.action", metrics.LastDays(1), "operation")
	be.NoError(t, err)
	be.AssertThat(t, groups["create"], be.Eq(float64(10)))
	be.AssertThat(t, groups["get"], be.Eq(float64(20)))
	be.AssertThat(t, store.pushGroupByCalls, be.Eq(1))
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
	be.NoError(t, err)
	be.AssertThat(t, count, be.Eq(int64(2)))
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
		be.AssertThat(t, result, be.Eq(tt.expected))
	}
}

func TestComputeBucket_Weekly(t *testing.T) {
	// 2026-02-16 is a Monday
	monday := time.Date(2026, 2, 16, 14, 0, 0, 0, time.UTC)
	result := metrics.ComputeBucket(monday, metrics.BucketWeekly)
	be.AssertThat(t, result, be.Eq("2026-02-16T00:00:00Z"))

	// Wednesday should still be in the same week
	wednesday := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)
	result = metrics.ComputeBucket(wednesday, metrics.BucketWeekly)
	be.AssertThat(t, result, be.Eq("2026-02-16T00:00:00Z"))
}

// ── Labels Tests ─────────────────────────────────────────────────────────

func TestLabels_Merge(t *testing.T) {
	a := metrics.Labels{"a": "1", "b": "2"}
	b := metrics.Labels{"b": "override", "c": "3"}

	merged := a.Merge(b)
	be.AssertThat(t, merged["a"], be.Eq("1"))
	be.AssertThat(t, merged["b"], be.Eq("override"))
	be.AssertThat(t, merged["c"], be.Eq("3"))
}

func TestLabels_MergeNil(t *testing.T) {
	var a metrics.Labels
	b := metrics.Labels{"x": "1"}

	merged := a.Merge(b)
	be.AssertThat(t, merged["x"], be.Eq("1"))

	merged2 := b.Merge(a)
	be.AssertThat(t, merged2["x"], be.Eq("1"))
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
	be.AssertThat(t, deleted, be.Eq(int64(2)))

	remaining := store.allRecords()
	be.RequireThat(t, remaining, be.HaveLength(1))
	be.AssertThat(t, remaining[0].ID, be.Eq("new1"))
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
	be.AssertThat(t, deleted, be.Eq(int64(2)))

	remaining := store.allRecords()
	be.RequireThat(t, remaining, be.HaveLength(3))
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
	be.AssertThat(t, deleted, be.Eq(int64(0)))
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
	be.AssertThat(t, deleted, be.Eq(int64(3)))
	be.AssertThat(t, created, be.Eq(int64(1)))

	remaining := store.allRecords()
	be.RequireThat(t, remaining, be.HaveLength(1))

	summary := remaining[0]
	be.AssertThat(t, summary.Value, be.Eq(float64(3)))
	be.AssertThat(t, summary.Labels.Val["_rollup"], be.Eq("true"))
	be.AssertThat(t, summary.Labels.Val["_rollup_count"], be.Eq("3"))
	expectedBucket := metrics.ComputeBucket(baseTime, metrics.BucketHourly)
	be.AssertThat(t, summary.Bucket, be.Eq(expectedBucket))
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
	be.AssertThat(t, deleted, be.Eq(int64(1)))
	be.AssertThat(t, created, be.Eq(int64(1)))

	remaining := store.allRecords()
	be.RequireThat(t, remaining, be.HaveLength(2))
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
	be.AssertThat(t, deleted, be.Eq(int64(2)))
	be.AssertThat(t, created, be.Eq(int64(2)))
}

// ── Actor Tests ──────────────────────────────────────────────────────────

func TestActor_ContextRoundTrip(t *testing.T) {
	ctx := context.Background()

	// Default is SystemActor
	actor := r3.GetActor(ctx)
	be.AssertThat(t, actor.Type, be.Eq("system"))

	// Set actor
	ctx = r3.WithActor(ctx, r3.Actor{ID: "99", Type: "admin"})
	actor = r3.GetActor(ctx)
	be.AssertThat(t, actor, be.HaveFields(map[string]any{"ID": "99", "Type": "admin"}))
}
