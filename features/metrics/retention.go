package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/amberpixels/years"

	"github.com/amberpixels/r3"
)

// RetentionPolicy defines cleanup rules for old metric records; both fields
// apply together, whichever deletes more.
type RetentionPolicy struct {
	// MaxAge deletes records older than this. Zero disables age-based retention.
	MaxAge time.Duration

	// MaxRecords caps records kept per entity type, deleting the oldest first.
	// Zero disables the count-based limit.
	MaxRecords int64
}

// RetentionEnforcer cleans up old metric records in any r3.CRUD[MetricRecord, string]
// per its policy.
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

// NewRetentionEnforcer creates an enforcer for store with the given policy.
func NewRetentionEnforcer(store r3.CRUD[MetricRecord, string], policy RetentionPolicy) *RetentionEnforcer {
	return &RetentionEnforcer{store: store, policy: policy}
}

// Enforce runs a single retention pass and returns the number of records
// deleted. Individual deletion errors are logged but do not halt the pass.
func (e *RetentionEnforcer) Enforce(ctx context.Context, recordType string) int64 {
	var deleted int64

	if e.policy.MaxAge > 0 {
		deleted += e.enforceMaxAge(ctx, recordType)
	}

	if e.policy.MaxRecords > 0 {
		deleted += e.enforceMaxRecords(ctx, recordType)
	}

	return deleted
}

// Start enforces retention periodically in a goroutine, returning a stop
// function that cancels the loop.
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
	cutoff := years.Now().UTC().Add(-e.policy.MaxAge)

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
	// QueryByType returns all records for this type sorted by created_at desc.
	q := QueryByType(recordType, TimeRange{
		From: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		To:   years.Now().UTC().Add(time.Hour), // generous future bound
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

	// Sorted desc, so the oldest excess records are at the tail.
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
