package metrics

import (
	"context"
)

// MetricCRUDError is the metric name for failed operation tracking.
const MetricCRUDError = "crud.error"

// ErrorCollector emits one "crud.error" entry (value 1) per failed operation.
// Unlike other collectors, it fires ONLY on error.
func ErrorCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err == nil {
			return nil
		}

		return []MetricEntry{{
			MetricName: MetricCRUDError,
			Value:      1,
		}}
	})
}
