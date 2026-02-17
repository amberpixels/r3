package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/amberpixels/r3"
)

// RetentionPolicy defines rules for automatic cleanup of old metric records.
// Multiple policies can be combined — the most aggressive one wins.
type RetentionPolicy struct {
	// MaxAge is the maximum age of metric records. Records older than this
	// are deleted during enforcement. Zero means no age-based retention.
	MaxAge time.Duration

	// MaxRecords is the maximum number of metric records to keep per entity type.
	// When exceeded, the oldest records are deleted first. Zero means no count-based limit.
	MaxRecords int64
}

// RetentionEnforcer runs cleanup of old metric records according to configured policies.
// It operates on any r3.CRUD[MetricRecord, string] store.
//
// Usage:
//
//	enforcer := metrics.NewRetentionEnforcer(metricsStore, metrics.RetentionPolicy{
//	    MaxAge: 90 * 24 * time.Hour,   // delete records older than 90 days
//	    MaxRecords: 1_000_000,          // keep at most 1M records per type
//	})
//
//	// Run once (e.g. from a cron job):
//	enforcer.Enforce(ctx, "orders")
//
//	// Run periodically in the background:
//	stop := enforcer.Start(ctx, "orders", 1*time.Hour)
//	defer stop()
type RetentionEnforcer struct {
	store  r3.CRUD[MetricRecord, string]
	policy RetentionPolicy
}

// NewRetentionEnforcer creates a new enforcer with the given policy.
func NewRetentionEnforcer(store r3.CRUD[MetricRecord, string], policy RetentionPolicy) *RetentionEnforcer {
	return &RetentionEnforcer{store: store, policy: policy}
}

// Enforce runs a single retention pass for the given record type.
// It deletes records that violate the policy and returns the number
// of records deleted. Errors during individual deletions are logged
// but do not stop the process.
func (e *RetentionEnforcer) Enforce(ctx context.Context, recordType string) int64 {
	var deleted int64

	// Age-based retention: delete records older than MaxAge.
	if e.policy.MaxAge > 0 {
		deleted += e.enforceMaxAge(ctx, recordType)
	}

	// Count-based retention: delete excess records.
	if e.policy.MaxRecords > 0 {
		deleted += e.enforceMaxRecords(ctx, recordType)
	}

	return deleted
}

// Start runs periodic retention enforcement in a background goroutine.
// Returns a stop function that cancels the background loop.
func (e *RetentionEnforcer) Start(ctx context.Context, recordType string, interval time.Duration) func() {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleted := e.Enforce(ctx, recordType)
				if deleted > 0 {
					slog.InfoContext(ctx, "r3metrics: retention enforced",
						"record_type", recordType,
						"deleted", deleted,
					)
				}
			}
		}
	}()

	return cancel
}

// enforceMaxAge deletes records older than the configured MaxAge.
func (e *RetentionEnforcer) enforceMaxAge(ctx context.Context, recordType string) int64 {
	cutoff := time.Now().UTC().Add(-e.policy.MaxAge)

	// Query for records older than the cutoff.
	q := r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.Fop(fieldCreatedAt, r3.OperatorLte, cutoff),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}

	records, _, err := e.store.List(ctx, q)
	if err != nil {
		slog.ErrorContext(ctx, "r3metrics: retention query failed",
			"record_type", recordType,
			"error", err,
		)
		return 0
	}

	return e.deleteRecords(ctx, records)
}

// enforceMaxRecords deletes the oldest records that exceed MaxRecords.
func (e *RetentionEnforcer) enforceMaxRecords(ctx context.Context, recordType string) int64 {
	// Fetch all records for this type (sorted by created_at desc via QueryByType).
	q := QueryByType(recordType, TimeRange{
		From: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Now().UTC().Add(time.Hour), // generous future bound
	})
	records, _, err := e.store.List(ctx, q)
	if err != nil {
		slog.ErrorContext(ctx, "r3metrics: retention count query failed",
			"record_type", recordType,
			"error", err,
		)
		return 0
	}

	if int64(len(records)) <= e.policy.MaxRecords {
		return 0
	}

	// Records are sorted by created_at desc, so the oldest are at the end.
	oldestRecords := records[e.policy.MaxRecords:]

	return e.deleteRecords(ctx, oldestRecords)
}

// deleteRecords deletes the given records one by one, logging errors.
func (e *RetentionEnforcer) deleteRecords(ctx context.Context, records []MetricRecord) int64 {
	var deleted int64
	for _, rec := range records {
		if err := e.store.Delete(ctx, rec.ID); err != nil {
			slog.ErrorContext(ctx, "r3metrics: retention delete failed",
				"record_id", rec.ID,
				"error", err,
			)
			continue
		}
		deleted++
	}
	return deleted
}
