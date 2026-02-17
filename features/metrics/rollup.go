package metrics

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/amberpixels/r3"
)

// RollupPolicy defines how to compact old fine-grained metric records into
// coarser summaries. For example, roll up minutely records older than 7 days
// into hourly summaries, or daily records older than 90 days into monthly.
type RollupPolicy struct {
	// SourceBucket is recorded in the summary's _rollup_source label for provenance.
	// It describes the original granularity of the compacted records
	// (e.g. BucketMinutely, BucketHourly).
	SourceBucket BucketSize

	// TargetBucket is the coarser bucket size to roll up into
	// (e.g. BucketHourly, BucketDaily).
	TargetBucket BucketSize

	// MinAge is the minimum age of records before they become eligible for rollup.
	// Only records older than this are compacted. Zero means all records.
	MinAge time.Duration
}

// RollupExecutor compacts fine-grained metric records into coarser summaries.
//
// The rollup process:
//  1. Query records matching the source bucket and age criteria.
//  2. Group by (record_type, metric_name, target_bucket, label set).
//  3. Sum values within each group into a single summary record.
//  4. Delete the original fine-grained records.
//  5. Insert the summary records.
//
// Usage:
//
//	executor := metrics.NewRollupExecutor(metricsStore, metrics.RollupPolicy{
//	    SourceBucket: metrics.BucketMinutely,
//	    TargetBucket: metrics.BucketHourly,
//	    MinAge:       7 * 24 * time.Hour,
//	})
//	executor.Execute(ctx, "orders")
type RollupExecutor struct {
	store  r3.CRUD[MetricRecord, string]
	policy RollupPolicy
}

// NewRollupExecutor creates a new rollup executor.
func NewRollupExecutor(store r3.CRUD[MetricRecord, string], policy RollupPolicy) *RollupExecutor {
	return &RollupExecutor{store: store, policy: policy}
}

// rollupKey identifies a group of records that should be summed together.
// Records with different label sets are kept separate to preserve dimensions.
type rollupKey struct {
	RecordType       string
	RecordID         string
	MetricName       string
	Bucket           string
	LabelFingerprint string // sorted "k=v;k=v" for consistent grouping.
}

// Execute runs a single rollup pass for the given record type.
// Returns the number of records deleted (compacted) and the number of summary records created.
func (rx *RollupExecutor) Execute(ctx context.Context, recordType string) (int64, int64) {
	cutoff := time.Now().UTC().Add(-rx.policy.MinAge)

	// Query old records.
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

	records, _, err := rx.store.List(ctx, q)
	if err != nil {
		slog.ErrorContext(ctx, "r3metrics: rollup query failed",
			"record_type", recordType,
			"error", err,
		)
		return 0, 0
	}

	if len(records) == 0 {
		return 0, 0
	}

	// Group records by (record_type, record_id, metric_name, target_bucket).
	// Within each group, sum values and merge labels.
	type groupData struct {
		sum    float64
		count  int64
		labels Labels
	}
	groups := make(map[rollupKey]*groupData)
	var deleteIDs []string

	for _, rec := range records {
		targetBucket := ComputeBucket(rec.CreatedAt, rx.policy.TargetBucket)
		key := rollupKey{
			RecordType:       rec.RecordType,
			RecordID:         rec.RecordID,
			MetricName:       rec.MetricName,
			Bucket:           targetBucket,
			LabelFingerprint: labelFingerprint(rec.Labels.Val),
		}

		g, exists := groups[key]
		if !exists {
			g = &groupData{labels: rec.Labels.Val.Clone()}
			groups[key] = g
		}
		g.sum += rec.Value
		g.count++
		deleteIDs = append(deleteIDs, rec.ID)
	}

	// Create summary records.
	var numCreated int64
	for key, g := range groups {
		// Add rollup metadata to labels.
		rollupLabels := g.labels.Merge(Labels{
			"_rollup":        "true",
			"_rollup_count":  strconv.FormatInt(g.count, 10),
			"_rollup_source": string(rx.policy.SourceBucket),
		})

		// Parse the target bucket as a timestamp for CreatedAt.
		bucketTime, parseErr := time.Parse(time.RFC3339, key.Bucket)
		if parseErr != nil {
			bucketTime = time.Now().UTC()
		}

		summary := MetricRecord{
			ID:         generateID(),
			RecordType: key.RecordType,
			RecordID:   key.RecordID,
			MetricName: key.MetricName,
			Value:      g.sum,
			Labels:     r3.NewJSONColumn(rollupLabels),
			Bucket:     key.Bucket,
			CreatedAt:  bucketTime,
		}

		if _, err := rx.store.Create(ctx, summary); err != nil {
			slog.ErrorContext(ctx, "r3metrics: rollup create failed",
				"metric_name", key.MetricName,
				"bucket", key.Bucket,
				"error", err,
			)
			continue
		}
		numCreated++
	}

	// Delete original fine-grained records.
	var numDeleted int64
	for _, id := range deleteIDs {
		if err := rx.store.Delete(ctx, id); err != nil {
			slog.ErrorContext(ctx, "r3metrics: rollup delete failed",
				"record_id", id,
				"error", err,
			)
			continue
		}
		numDeleted++
	}

	return numDeleted, numCreated
}

// labelFingerprint produces a stable string key from a Labels map
// for use as a map key during rollup grouping. Keys are sorted
// alphabetically to ensure deterministic output.
func labelFingerprint(l Labels) string {
	if len(l) == 0 {
		return ""
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(';')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(l[k])
	}
	return b.String()
}
