package metrics

import "context"

// AggregationPusher is an optional interface that metric stores can implement
// to provide server-side aggregation instead of in-memory computation.
//
// Backends like Elasticsearch, ClickHouse, or TimescaleDB have native aggregation
// capabilities that are far more efficient than fetching all records and
// computing in Go. If a store implements AggregationPusher, the Aggregator
// will delegate to these methods instead of performing in-memory aggregation.
//
// This is a pure opt-in: existing r3.CRUD[MetricRecord, string] stores continue
// to work with the in-memory Aggregator. Only stores that additionally implement
// this interface get the server-side performance benefit.
//
// Usage:
//
//	// A custom store implementing both r3.CRUD and AggregationPusher:
//	type ClickHouseMetricStore struct { ... }
//	func (s *ClickHouseMetricStore) PushCount(...) (int64, error) { ... }
//	func (s *ClickHouseMetricStore) PushSum(...)   (float64, error) { ... }
//	// etc.
//
//	agg := metrics.NewAggregator(clickhouseStore)
//	// Aggregator automatically detects the AggregationPusher and delegates.
type AggregationPusher interface {
	// PushCount returns the count of metric records matching the given criteria.
	// The labels parameter is optional — if non-nil, only records with those labels are counted.
	PushCount(ctx context.Context, recordType, metricName string, tr TimeRange, labels Labels) (int64, error)

	// PushSum returns the sum of metric values matching the given criteria.
	PushSum(ctx context.Context, recordType, metricName string, tr TimeRange, labels Labels) (float64, error)

	// PushAvg returns the average of metric values matching the given criteria.
	PushAvg(ctx context.Context, recordType, metricName string, tr TimeRange, labels Labels) (float64, error)

	// PushTopN returns the top N entities by summed metric value.
	PushTopN(
		ctx context.Context,
		recordType, metricName string,
		tr TimeRange,
		n int,
		labels Labels,
	) ([]RankedEntity, error)

	// PushTimeSeries returns metric values grouped by time bucket.
	PushTimeSeries(
		ctx context.Context,
		recordType, metricName string,
		tr TimeRange,
		bucket BucketSize,
		labels Labels,
	) ([]BucketValue, error)

	// PushGroupBy returns metric values grouped by a label key.
	PushGroupBy(
		ctx context.Context,
		recordType, metricName string,
		tr TimeRange,
		labelKey string,
		labels Labels,
	) (map[string]float64, error)
}
