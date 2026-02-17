package metrics

import (
	"context"
)

// MetricCRUDError is the metric name for failed operation tracking.
const MetricCRUDError = "crud.error"

// ErrorCollector tracks failed CRUD operations.
// Emits one "crud.error" entry with value 1 when any CRUD operation returns an error.
//
// Unlike other collectors, this fires ONLY on error (Err != nil).
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
