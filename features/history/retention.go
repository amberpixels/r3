package history

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/amberpixels/r3"
)

// RetentionPolicy defines rules for automatic cleanup of old change records.
type RetentionPolicy struct {
	// MaxAge is the maximum age of change records. Records older than this
	// are deleted during enforcement. Zero means no age-based retention.
	MaxAge time.Duration

	// MaxVersions is the maximum number of versions to keep per entity.
	// When exceeded, the oldest versions are deleted first.
	// Zero means no version-based limit.
	MaxVersions int64
}

// RetentionEnforcer runs cleanup of old change records according to configured policies.
//
// Usage:
//
//	enforcer := history.NewRetentionEnforcer(historyStore, history.RetentionPolicy{
//	    MaxAge:      90 * 24 * time.Hour,  // delete records older than 90 days
//	    MaxVersions: 100,                   // keep at most 100 versions per entity
//	})
//
//	enforcer.Enforce(ctx, "orders")
type RetentionEnforcer struct {
	store  r3.CRUD[ChangeRecord, string]
	policy RetentionPolicy
}

// NewRetentionEnforcer creates a new enforcer with the given policy.
func NewRetentionEnforcer(store r3.CRUD[ChangeRecord, string], policy RetentionPolicy) *RetentionEnforcer {
	return &RetentionEnforcer{store: store, policy: policy}
}

// Enforce runs a single retention pass for the given record type.
// Returns the number of records deleted.
func (e *RetentionEnforcer) Enforce(ctx context.Context, recordType string) int64 {
	var deleted int64

	if e.policy.MaxAge > 0 {
		deleted += e.enforceMaxAge(ctx, recordType)
	}

	if e.policy.MaxVersions > 0 {
		deleted += e.enforceMaxVersions(ctx, recordType)
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
					slog.InfoContext(ctx, "r3history: retention enforced",
						"record_type", recordType,
						"deleted", deleted,
					)
				}
			}
		}
	}()

	return cancel
}

// enforceMaxAge deletes change records older than MaxAge.
func (e *RetentionEnforcer) enforceMaxAge(ctx context.Context, recordType string) int64 {
	cutoff := time.Now().UTC().Add(-e.policy.MaxAge)

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
		slog.ErrorContext(ctx, "r3history: retention query failed",
			"record_type", recordType,
			"error", err,
		)
		return 0
	}

	return e.deleteRecords(ctx, records)
}

// enforceMaxVersions deletes versions exceeding MaxVersions per entity.
// It lists all entities of the type, then for each entity with more than
// MaxVersions, deletes the oldest versions.
func (e *RetentionEnforcer) enforceMaxVersions(ctx context.Context, recordType string) int64 {
	// Get all records for this type.
	q := QueryForType(recordType)
	records, _, err := e.store.List(ctx, q)
	if err != nil {
		slog.ErrorContext(ctx, "r3history: retention version query failed",
			"record_type", recordType,
			"error", err,
		)
		return 0
	}

	// Group by record_id.
	groups := make(map[string][]ChangeRecord)
	for _, rec := range records {
		groups[rec.RecordID] = append(groups[rec.RecordID], rec)
	}

	var deleted int64
	for _, recs := range groups {
		if int64(len(recs)) <= e.policy.MaxVersions {
			continue
		}
		// Sort by version ascending so we can delete the oldest.
		sort.Slice(recs, func(i, j int) bool {
			return recs[i].Version < recs[j].Version
		})
		// Keep the latest MaxVersions, delete the rest.
		excess := int64(len(recs)) - e.policy.MaxVersions
		deleted += e.deleteRecords(ctx, recs[:excess])
	}

	return deleted
}

// deleteRecords deletes the given records one by one.
func (e *RetentionEnforcer) deleteRecords(ctx context.Context, records []ChangeRecord) int64 {
	var deleted int64
	for _, rec := range records {
		if err := e.store.Delete(ctx, rec.ID); err != nil {
			slog.ErrorContext(ctx, "r3history: retention delete failed",
				"record_id", rec.ID,
				"error", err,
			)
			continue
		}
		deleted++
	}
	return deleted
}
