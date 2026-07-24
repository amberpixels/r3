package history

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/amberpixels/years"

	"github.com/amberpixels/r3"
)

// RetentionPolicy defines automatic cleanup rules for old change records. A zero
// field disables that dimension.
type RetentionPolicy struct {
	// MaxAge deletes records older than this during enforcement.
	MaxAge time.Duration

	// MaxVersions caps versions kept per entity; the oldest are pruned first.
	MaxVersions int64
}

// RetentionEnforcer cleans up old change records per a [RetentionPolicy].
//
//	enforcer := history.NewRetentionEnforcer(historyStore, history.RetentionPolicy{
//	    MaxAge:      90 * 24 * time.Hour,
//	    MaxVersions: 100,
//	})
//	enforcer.Enforce(ctx, "orders")
type RetentionEnforcer struct {
	store  r3.CRUD[ChangeRecord, string]
	policy RetentionPolicy
}

// NewRetentionEnforcer creates an enforcer with the given policy.
func NewRetentionEnforcer(store r3.CRUD[ChangeRecord, string], policy RetentionPolicy) *RetentionEnforcer {
	return &RetentionEnforcer{store: store, policy: policy}
}

// Enforce runs one retention pass for a record type, returning the count deleted.
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

// Start runs periodic enforcement in a background goroutine, returning a stop
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

// enforceMaxAge prunes records older than MaxAge. Rather than deleting outright -
// which would strip the v1 baseline (and intermediate diffs) that Reconstruct/
// RevertTo replay from - it folds each entity's pruned versions into its oldest
// survivor, keeping that record a self-sufficient full baseline.
func (e *RetentionEnforcer) enforceMaxAge(ctx context.Context, recordType string) int64 {
	cutoff := years.Now().UTC().Add(-e.policy.MaxAge)

	// Fetch all versions so, per entity, we see both the pre-cutoff versions to
	// prune and the survivors to compact into.
	records, _, err := e.store.List(ctx, QueryForType(recordType))
	if err != nil {
		slog.ErrorContext(ctx, "r3history: retention query failed",
			"record_type", recordType,
			"error", err,
		)
		return 0
	}

	var deleted int64
	for _, recs := range groupByRecordSortedByVersion(records) {
		// Versions are monotonic in time, so the pre-cutoff records form the
		// oldest contiguous prefix.
		deleteCount := 0
		for _, rec := range recs {
			if rec.CreatedAt.After(cutoff) {
				break
			}
			deleteCount++
		}
		deleted += e.compactAndPrune(ctx, recs, deleteCount)
	}

	return deleted
}

// enforceMaxVersions prunes versions above MaxVersions per entity, folding the
// pruned versions into the oldest survivor so it stays a full baseline (see
// compactAndPrune).
func (e *RetentionEnforcer) enforceMaxVersions(ctx context.Context, recordType string) int64 {
	records, _, err := e.store.List(ctx, QueryForType(recordType))
	if err != nil {
		slog.ErrorContext(ctx, "r3history: retention version query failed",
			"record_type", recordType,
			"error", err,
		)
		return 0
	}

	var deleted int64
	for _, recs := range groupByRecordSortedByVersion(records) {
		if int64(len(recs)) <= e.policy.MaxVersions {
			continue
		}
		excess := int(int64(len(recs)) - e.policy.MaxVersions)
		deleted += e.compactAndPrune(ctx, recs, excess)
	}

	return deleted
}

// groupByRecordSortedByVersion groups change records by RecordID and sorts each
// group ascending by version.
func groupByRecordSortedByVersion(records []ChangeRecord) map[string][]ChangeRecord {
	groups := make(map[string][]ChangeRecord)
	for _, rec := range records {
		groups[rec.RecordID] = append(groups[rec.RecordID], rec)
	}
	for _, recs := range groups {
		sort.Slice(recs, func(i, j int) bool { return recs[i].Version < recs[j].Version })
	}
	return groups
}

// compactAndPrune removes the oldest deleteCount versions from one entity's
// version-ascending history without breaking diff-based reconstruction.
//
// Reconstruct replays diffs from a full baseline (normally v1), so deleting the
// baseline - or any intermediate version - would leave the oldest survivor a
// partial diff and corrupt every revert. So the pruned versions plus the oldest
// survivor are folded into a single full-state baseline that replaces the
// survivor's Changes; the survivor keeps its own version, id, and timestamp.
//
// If the baseline write fails, nothing is deleted - redundant history beats a
// corrupt chain. Returns the number of records deleted.
func (e *RetentionEnforcer) compactAndPrune(ctx context.Context, recs []ChangeRecord, deleteCount int) int64 {
	if deleteCount <= 0 {
		return 0
	}
	if deleteCount >= len(recs) {
		// The whole history expired - no survivor to preserve a baseline for.
		return e.deleteRecords(ctx, recs)
	}

	survivor := recs[deleteCount]
	survivor.Changes = r3.NewJSONColumn(foldBaseline(recs[:deleteCount+1]))
	if _, err := e.store.Update(ctx, survivor); err != nil {
		slog.ErrorContext(ctx,
			"r3history: retention compaction failed; skipping prune to keep reconstruction intact",
			"record_type", survivor.RecordType,
			"record_id", survivor.RecordID,
			"error", err,
		)
		return 0
	}

	return e.deleteRecords(ctx, recs[:deleteCount])
}

// foldBaseline collapses a version-ascending slice of one entity's records into a
// full-state baseline: every field that ever held a value appears once with
// OldValue=nil and NewValue=latest. This mirrors a v1 create, so Reconstruct
// replays from it unchanged.
func foldBaseline(recs []ChangeRecord) []FieldChange {
	state := make(map[string]any)
	order := make([]string, 0)
	for _, rec := range recs {
		for _, ch := range rec.Changes.Val {
			if _, seen := state[ch.Field]; !seen {
				order = append(order, ch.Field)
			}
			state[ch.Field] = ch.NewValue
		}
	}

	baseline := make([]FieldChange, 0, len(order))
	for _, field := range order {
		baseline = append(baseline, FieldChange{Field: field, OldValue: nil, NewValue: state[field]})
	}
	return baseline
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
