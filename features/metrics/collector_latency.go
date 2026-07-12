package metrics

import (
	"context"
)

// MetricCRUDLatency is the metric name for operation duration; value is
// milliseconds.
const MetricCRUDLatency = "crud.action.latency"

// LatencyCollector emits one "crud.action.latency" entry per operation, valued
// as wall-clock milliseconds. Fires on both success and failure.
func LatencyCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		return []MetricEntry{{
			MetricName: MetricCRUDLatency,
			Value:      float64(opCtx.Duration.Milliseconds()),
		}}
	})
}
