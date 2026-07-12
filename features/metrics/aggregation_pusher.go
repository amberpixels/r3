package metrics

import "context"

// AggregationPusher is an optional interface a metric store implements to push
// aggregation server-side instead of fetching all records and computing in Go.
// If a store implements it, [Aggregator] delegates to these methods; otherwise
// it falls back to in-memory aggregation, so this is pure opt-in.
type AggregationPusher interface {
	// PushCount returns the count of matching records. A non-nil labels filters
	// to records carrying those labels.
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
