package metrics

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/years"
)

// RollupPolicy defines how old fine-grained records are compacted into coarser
// summaries - e.g. minutely records older than 7 days into hourly.
type RollupPolicy struct {
	// SourceBucket is recorded in the summary's _rollup_source label for
	// provenance only; it does not filter which records are compacted.
	SourceBucket BucketSize

	// TargetBucket is the coarser bucket size to roll up into.
	TargetBucket BucketSize

	// MinAge compacts only records older than this. Zero means all records.
	MinAge time.Duration
}

// RollupExecutor compacts fine-grained metric records into coarser summaries by
// querying old records, grouping by (record_type, metric_name, target_bucket,
// label set), summing each group into one summary, then deleting the originals.
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

// NewRollupExecutor creates a rollup executor for store with the given policy.
func NewRollupExecutor(store r3.CRUD[MetricRecord, string], policy RollupPolicy) *RollupExecutor {
	return &RollupExecutor{store: store, policy: policy}
}

// rollupKey identifies a group of records summed together. Distinct label sets
// stay separate to preserve dimensions.
type rollupKey struct {
	RecordType       string
	RecordID         string
	MetricName       string
	Bucket           string
	LabelFingerprint string // sorted "k=v;k=v" for consistent grouping.
}

// Execute runs a single rollup pass, returning the count of records deleted
// (compacted) and summary records created.
func (rx *RollupExecutor) Execute(ctx context.Context, recordType string) (int64, int64) {
	cutoff := years.Now().UTC().Add(-rx.policy.MinAge)

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

	// Group, then sum values and merge labels within each group.
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

	var numCreated int64
	for key, g := range groups {
		rollupLabels := g.labels.Merge(Labels{
			"_rollup":        "true",
			"_rollup_count":  strconv.FormatInt(g.count, 10),
			"_rollup_source": string(rx.policy.SourceBucket),
		})

		// The target bucket string doubles as CreatedAt.
		bucketTime, parseErr := years.Parse(time.RFC3339, key.Bucket)
		if parseErr != nil {
			bucketTime = years.Now().UTC()
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

// labelFingerprint produces a deterministic map key from a Labels set, sorting
// keys alphabetically for consistent grouping.
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
