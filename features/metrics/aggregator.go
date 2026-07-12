package metrics

import (
	"context"
	"sort"

	"github.com/amberpixels/r3"
)

// RankedEntity is an entity ranked by an aggregated metric value.
type RankedEntity struct {
	RecordID string
	Value    float64
}

// BucketValue is a time bucket with its aggregated metric value.
type BucketValue struct {
	Bucket string
	Value  float64
}

// Aggregator computes metric aggregations over any [r3.Querier] for MetricRecord
// (read access only). It aggregates in-memory unless the store implements
// AggregationPusher, in which case it delegates server-side.
type Aggregator struct {
	store  r3.Querier[MetricRecord, string]
	pusher AggregationPusher // nil if store doesn't implement AggregationPusher.
}

// NewAggregator creates an Aggregator over the given metric store, enabling
// server-side aggregation automatically if the store implements AggregationPusher.
func NewAggregator(store r3.Querier[MetricRecord, string]) *Aggregator {
	a := &Aggregator{store: store}
	if p, ok := store.(AggregationPusher); ok {
		a.pusher = p
	}
	return a
}

// Count returns the number of metric records matching the query. For counter
// metrics (value=1) this equals the sum.
//
//	agg.Count(ctx, "users", "crud.action", LastMonth(), WithLabel("operation", "create"))
func (a *Aggregator) Count(
	ctx context.Context,
	recordType, metricName string,
	tr TimeRange,
	opts ...QueryOption,
) (int64, error) {
	if a.pusher != nil {
		return a.pusher.PushCount(ctx, recordType, metricName, tr, resolveLabels(opts))
	}
	records, err := a.fetch(ctx, recordType, metricName, tr, opts...)
	if err != nil {
		return 0, err
	}
	return int64(len(records)), nil
}

// Sum returns the sum of metric values matching the query.
//
//	agg.Sum(ctx, "users", "crud.action.latency", LastWeek(), WithLabel("operation", "list"))
func (a *Aggregator) Sum(
	ctx context.Context,
	recordType, metricName string,
	tr TimeRange,
	opts ...QueryOption,
) (float64, error) {
	if a.pusher != nil {
		return a.pusher.PushSum(ctx, recordType, metricName, tr, resolveLabels(opts))
	}
	records, err := a.fetch(ctx, recordType, metricName, tr, opts...)
	if err != nil {
		return 0, err
	}
	var total float64
	for _, r := range records {
		total += r.Value
	}
	return total, nil
}

// Avg returns the average metric value matching the query.
//
//	agg.Avg(ctx, "users", "crud.action.latency", LastWeek(), WithLabel("operation", "create"))
func (a *Aggregator) Avg(
	ctx context.Context,
	recordType, metricName string,
	tr TimeRange,
	opts ...QueryOption,
) (float64, error) {
	if a.pusher != nil {
		return a.pusher.PushAvg(ctx, recordType, metricName, tr, resolveLabels(opts))
	}
	records, err := a.fetch(ctx, recordType, metricName, tr, opts...)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	var total float64
	for _, r := range records {
		total += r.Value
	}
	return total / float64(len(records)), nil
}

// TopN returns the N entities with the highest summed metric value.
//
//	agg.TopN(ctx, "users", "entity.popularity", LastMonth(), 10)
func (a *Aggregator) TopN(
	ctx context.Context,
	recordType, metricName string,
	tr TimeRange,
	n int,
	opts ...QueryOption,
) ([]RankedEntity, error) {
	if a.pusher != nil {
		return a.pusher.PushTopN(ctx, recordType, metricName, tr, n, resolveLabels(opts))
	}
	records, err := a.fetch(ctx, recordType, metricName, tr, opts...)
	if err != nil {
		return nil, err
	}

	sums := make(map[string]float64)
	for _, r := range records {
		if r.RecordID != "" {
			sums[r.RecordID] += r.Value
		}
	}

	ranked := make([]RankedEntity, 0, len(sums))
	for id, val := range sums {
		ranked = append(ranked, RankedEntity{RecordID: id, Value: val})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Value > ranked[j].Value
	})

	if n > 0 && n < len(ranked) {
		ranked = ranked[:n]
	}
	return ranked, nil
}

// TimeSeries returns metric values grouped by time bucket.
//
//	agg.TimeSeries(ctx, "users", "crud.action", Last30Days(), BucketDaily, WithLabel("operation", "create"))
func (a *Aggregator) TimeSeries(
	ctx context.Context,
	recordType, metricName string,
	tr TimeRange,
	bucket BucketSize,
	opts ...QueryOption,
) ([]BucketValue, error) {
	if a.pusher != nil {
		return a.pusher.PushTimeSeries(ctx, recordType, metricName, tr, bucket, resolveLabels(opts))
	}
	records, err := a.fetch(ctx, recordType, metricName, tr, opts...)
	if err != nil {
		return nil, err
	}

	// Re-bucket to the requested size; records may have been stored at a different one.
	sums := make(map[string]float64)
	var order []string
	for _, r := range records {
		b := ComputeBucket(r.CreatedAt, bucket)
		if _, exists := sums[b]; !exists {
			order = append(order, b)
		}
		sums[b] += r.Value
	}

	sort.Strings(order) // chronological

	result := make([]BucketValue, 0, len(order))
	for _, b := range order {
		result = append(result, BucketValue{Bucket: b, Value: sums[b]})
	}
	return result, nil
}

// GroupBy returns metric values grouped by a label key.
//
//	agg.GroupBy(ctx, "orders", "crud.action", LastMonth(), "operation")
//	// -> {"create": 1247, "get": 8391, "list": 2100, ...}
func (a *Aggregator) GroupBy(
	ctx context.Context,
	recordType, metricName string,
	tr TimeRange,
	labelKey string,
	opts ...QueryOption,
) (map[string]float64, error) {
	if a.pusher != nil {
		return a.pusher.PushGroupBy(ctx, recordType, metricName, tr, labelKey, resolveLabels(opts))
	}
	records, err := a.fetch(ctx, recordType, metricName, tr, opts...)
	if err != nil {
		return nil, err
	}

	groups := make(map[string]float64)
	for _, r := range records {
		key := r.Labels.Val[labelKey]
		if key == "" {
			key = "(none)"
		}
		groups[key] += r.Value
	}
	return groups, nil
}

// QueryOption configures additional filtering for aggregator methods.
type QueryOption func(*queryConfig)

type queryConfig struct {
	labels Labels
}

// WithLabel restricts an aggregator query to records carrying this exact
// label key-value pair.
func WithLabel(key, value string) QueryOption {
	return func(qc *queryConfig) {
		if qc.labels == nil {
			qc.labels = make(Labels)
		}
		qc.labels[key] = value
	}
}

// fetch retrieves matching metric records from the store and applies
// in-memory label filtering.
func (a *Aggregator) fetch(
	ctx context.Context,
	recordType, metricName string,
	tr TimeRange,
	opts ...QueryOption,
) ([]MetricRecord, error) {
	var qc queryConfig
	for _, opt := range opts {
		opt(&qc)
	}

	q := QueryByMetric(recordType, metricName, tr)
	records, _, err := a.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	if len(qc.labels) > 0 {
		filtered := records[:0]
		for _, r := range records {
			if matchesLabels(r.Labels.Val, qc.labels) {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}

	return records, nil
}

// matchesLabels returns true if the record's labels contain all required labels.
func matchesLabels(recordLabels, required Labels) bool {
	for k, v := range required {
		if recordLabels[k] != v {
			return false
		}
	}
	return true
}

// resolveLabels collects the label filters from opts, or nil if none were set.
func resolveLabels(opts []QueryOption) Labels {
	var qc queryConfig
	for _, opt := range opts {
		opt(&qc)
	}
	return qc.labels
}
