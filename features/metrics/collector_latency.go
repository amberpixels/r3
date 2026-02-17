package metrics

import (
	"context"
)

// MetricCRUDLatency is the metric name for operation duration tracking.
// Value is the duration in milliseconds.
const MetricCRUDLatency = "crud.action.latency"

// LatencyCollector records the wall-clock duration of each CRUD operation.
// Emits one "crud.action.latency" entry with value = duration in milliseconds.
//
// Fires on both success and failure — latency is always interesting.
func LatencyCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		return []MetricEntry{{
			MetricName: MetricCRUDLatency,
			Value:      float64(opCtx.Duration.Milliseconds()),
		}}
	})
}
