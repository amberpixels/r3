package metrics

import (
	"context"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/amberpixels/r3"
)

// idCounter ensures unique IDs even for records created in the same nanosecond.
var idCounter atomic.Int64

// CRUD records metrics for every operation via configurable collectors, storing
// them in any r3.CRUD[MetricRecord, string]. See the package doc for details.
type CRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
	store r3.CRUD[MetricRecord, string]
	opts  Options[T, ID]
}

var (
	_ r3.CRUD[any, any]     = &CRUD[any, any]{}
	_ r3.Aggregator         = &CRUD[any, any]{}
	_ r3.RelationAggregator = &CRUD[any, any]{}
)

// WithMetrics wraps inner with metrics collection persisted to store.
func WithMetrics[T any, ID comparable](
	inner r3.CRUD[T, ID],
	store r3.CRUD[MetricRecord, string],
	optFns ...Option[T, ID],
) *CRUD[T, ID] {
	var opts Options[T, ID]
	for _, fn := range optFns {
		fn(&opts)
	}
	applyDefaults(&opts)

	return &CRUD[T, ID]{
		inner: inner,
		store: store,
		opts:  opts,
	}
}

// Inner returns the underlying CRUD, bypassing metrics for a specific operation.
func (m *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return m.inner
}

// Unwrap returns the wrapped CRUD so the decorator chain can be walked.
func (m *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return m.inner
}

// Rewrap rebuilds this decorator around a different inner CRUD, sharing the
// store and options (used to re-apply metrics over a transaction-bound CRUD).
func (m *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner, store: m.store, opts: m.opts}
}

// Metrics returns the metric record CRUD for querying metrics directly,
// e.g. via the query builders or the Aggregator.
func (m *CRUD[T, ID]) Metrics() r3.CRUD[MetricRecord, string] {
	return m.store
}

// Create inserts a new entity and records metrics.
func (m *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	start := time.Now()
	result, err := m.inner.Create(ctx, entity)
	duration := time.Since(start)

	opCtx := OperationContext[T, ID]{
		Operation: OpCreate,
		Duration:  duration,
		Entity:    result,
		Err:       err,
	}
	if err == nil && m.opts.IDFunc != nil {
		opCtx.EntityID = m.opts.IDFunc(result)
	}
	m.record(ctx, opCtx)

	return result, err
}

// Get retrieves an entity by ID and records metrics.
func (m *CRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var q r3.Query
	if len(qarg) > 0 {
		q = qarg[0]
	}

	start := time.Now()
	result, err := m.inner.Get(ctx, id, qarg...)
	duration := time.Since(start)

	m.record(ctx, OperationContext[T, ID]{
		Operation: OpGet,
		Duration:  duration,
		Entity:    result,
		EntityID:  id,
		Query:     q,
		Err:       err,
	})

	return result, err
}

// List retrieves entities and records metrics.
func (m *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	var q r3.Query
	if len(qarg) > 0 {
		q = qarg[0]
	}

	start := time.Now()
	results, total, err := m.inner.List(ctx, qarg...)
	duration := time.Since(start)

	m.record(ctx, OperationContext[T, ID]{
		Operation:  OpList,
		Duration:   duration,
		Entities:   results,
		TotalCount: total,
		Query:      q,
		Err:        err,
	})

	return results, total, err
}

// Count returns the number of matching entities and records metrics.
func (m *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	var q r3.Query
	if len(qarg) > 0 {
		q = qarg[0]
	}

	start := time.Now()
	total, err := m.inner.Count(ctx, qarg...)
	duration := time.Since(start)

	m.record(ctx, OperationContext[T, ID]{
		Operation:  OpCount,
		Duration:   duration,
		TotalCount: total,
		Query:      q,
		Err:        err,
	})

	return total, err
}

// Aggregate computes grouped aggregates and records metrics (OpAggregate,
// with TotalCount = number of grouped rows returned). An inner repo without
// aggregation support yields r3.ErrAggregateNotSupported.
func (m *CRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	agg, ok := m.inner.(r3.Aggregator)
	if !ok {
		return nil, r3.ErrAggregateNotSupported
	}

	var q r3.Query
	if len(qarg) > 0 {
		q = qarg[0]
	}

	start := time.Now()
	rows, err := agg.Aggregate(ctx, qarg...)
	duration := time.Since(start)

	m.record(ctx, OperationContext[T, ID]{
		Operation:  OpAggregate,
		Duration:   duration,
		TotalCount: int64(len(rows)),
		Query:      q,
		Err:        err,
	})

	return rows, err
}

// AggregateThroughRelation aggregates related rows and records metrics
// (OpAggregate, with TotalCount = number of grouped rows returned). An inner
// repo without relation-aggregation support yields
// r3.ErrRelationAggregateNotSupported.
func (m *CRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	agg, ok := m.inner.(r3.RelationAggregator)
	if !ok {
		return nil, r3.ErrRelationAggregateNotSupported
	}

	var q r3.Query
	if len(qarg) > 0 {
		q = qarg[0]
	}

	start := time.Now()
	rows, err := agg.AggregateThroughRelation(ctx, relation, qarg...)
	duration := time.Since(start)

	m.record(ctx, OperationContext[T, ID]{
		Operation:  OpAggregate,
		Duration:   duration,
		TotalCount: int64(len(rows)),
		Query:      q,
		Err:        err,
	})

	return rows, err
}

// Update modifies an existing entity and records metrics.
func (m *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	// Fetch old state for diff-based collectors
	var old T
	var hasOld bool
	if m.opts.IDFunc != nil {
		id := m.opts.IDFunc(entity)
		oldEntity, err := m.inner.Get(ctx, id)
		if err == nil {
			old = oldEntity
			hasOld = true
		}
	}

	start := time.Now()
	result, err := m.inner.Update(ctx, entity)
	duration := time.Since(start)

	opCtx := OperationContext[T, ID]{
		Operation: OpUpdate,
		Duration:  duration,
		Entity:    result,
		OldEntity: old,
		HasOld:    hasOld,
		Err:       err,
	}
	if m.opts.IDFunc != nil {
		opCtx.EntityID = m.opts.IDFunc(entity)
	}
	m.record(ctx, opCtx)

	return result, err
}

// Patch performs a partial update and records metrics.
func (m *CRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	// Fetch old state for diff-based collectors
	var old T
	var hasOld bool
	if m.opts.IDFunc != nil {
		id := m.opts.IDFunc(entity)
		oldEntity, err := m.inner.Get(ctx, id)
		if err == nil {
			old = oldEntity
			hasOld = true
		}
	}

	start := time.Now()
	result, err := m.inner.Patch(ctx, entity, fields)
	duration := time.Since(start)

	opCtx := OperationContext[T, ID]{
		Operation: OpPatch,
		Duration:  duration,
		Entity:    result,
		OldEntity: old,
		HasOld:    hasOld,
		Fields:    fields,
		Err:       err,
	}
	if m.opts.IDFunc != nil {
		opCtx.EntityID = m.opts.IDFunc(entity)
	}
	m.record(ctx, opCtx)

	return result, err
}

// Delete removes an entity and records metrics.
func (m *CRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	// Fetch entity before deletion for labelers and lifecycle metrics
	var old T
	var hasOld bool
	oldEntity, err := m.inner.Get(ctx, id)
	if err == nil {
		old = oldEntity
		hasOld = true
	}

	start := time.Now()
	err = m.inner.Delete(ctx, id)
	duration := time.Since(start)

	m.record(ctx, OperationContext[T, ID]{
		Operation: OpDelete,
		Duration:  duration,
		Entity:    old,
		OldEntity: old,
		HasOld:    hasOld,
		EntityID:  id,
		Err:       err,
	})

	return err
}

// record runs all collectors, merges labels, and persists MetricRecords.
// If Async is enabled, recording runs in a background goroutine.
func (m *CRUD[T, ID]) record(ctx context.Context, opCtx OperationContext[T, ID]) {
	if len(m.opts.Collectors) == 0 {
		return
	}

	doRecord := func(recordCtx context.Context) {
		// Everything below reads from recordCtx, never the captured ctx: in async
		// mode recordCtx is a detached (WithoutCancel) copy, so collectors, actor
		// extraction, and context labelers must use it too - otherwise they would
		// observe the request context after it has been cancelled.

		var entries []MetricEntry
		for _, c := range m.opts.Collectors {
			entries = append(entries, c.Collect(recordCtx, opCtx)...)
		}
		if len(entries) == 0 {
			return
		}

		// Core labels are auto-injected by the framework.
		actor := r3.GetActor(recordCtx)
		coreLabels := Labels{
			"operation":  string(opCtx.Operation),
			"actor_id":   actor.ID,
			"actor_type": actor.Type,
		}

		var ctxLabels Labels
		for _, cl := range m.opts.ContextLabelers {
			ctxLabels = ctxLabels.Merge(cl(recordCtx))
		}

		var entityLabels Labels
		hasEntity := opCtx.Operation != OpList // List has multiple entities; labelers run per-entry
		if hasEntity && len(m.opts.Labelers) > 0 {
			for _, l := range m.opts.Labelers {
				entityLabels = entityLabels.Merge(l(opCtx.Entity))
			}
		}

		now := time.Now()
		bucket := ComputeBucket(now, m.opts.BucketSize)

		for _, entry := range entries {
			// core -> context -> entity -> collector-specific; more specific wins.
			merged := coreLabels.Merge(ctxLabels).Merge(entityLabels).Merge(entry.Labels)

			record := MetricRecord{
				ID:         generateID(),
				RecordType: m.opts.RecordType,
				RecordID:   entry.RecordID,
				MetricName: entry.MetricName,
				Value:      entry.Value,
				Labels:     r3.NewJSONColumn(merged),
				Bucket:     bucket,
				CreatedAt:  now,
			}

			if _, err := m.store.Create(recordCtx, record); err != nil {
				m.handleError(recordCtx, err)
			}
		}
	}

	if m.opts.Async {
		// Detach from the request lifetime (so recording isn't cancelled when the
		// request returns) while preserving request-scoped values like the Actor.
		go doRecord(context.WithoutCancel(ctx))
		return
	}

	doRecord(ctx)
}

// handleError handles metric persistence errors.
func (m *CRUD[T, ID]) handleError(ctx context.Context, err error) {
	if m.opts.ErrorHandler != nil {
		m.opts.ErrorHandler(err)
		return
	}
	slog.ErrorContext(ctx, "r3metrics: record failed", "record_type", m.opts.RecordType, "error", err)
}

// generateID creates a unique ID for a metric record.
func generateID() string {
	seq := idCounter.Add(1)
	return strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.FormatInt(seq, 10)
}
