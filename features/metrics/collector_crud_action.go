package metrics

import (
	"context"
	"fmt"
)

// MetricCRUDAction is the metric name for the universal CRUD action counter,
// incremented by 1 on every operation. The "operation" core label distinguishes
// create/get/list/update/patch/delete.
const MetricCRUDAction = "crud.action"

// CRUDActionCollector emits one "crud.action" entry (value 1) per successful
// operation. The operation type rides the decorator's "operation" core label,
// so this collector does not add it. RecordID is set for entity-level ops and
// empty for type-level ones (list/count/aggregate).
func CRUDActionCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil {
			return nil
		}

		entry := MetricEntry{
			MetricName: MetricCRUDAction,
			Value:      1,
		}

		switch opCtx.Operation {
		case OpCreate, OpGet, OpUpdate, OpPatch, OpDelete, OpUpsert:
			entry.RecordID = fmt.Sprint(opCtx.EntityID)
		case OpList, OpCount, OpAggregate, OpPatchWhere:
			// Type-level ops span many or no single rows; RecordID stays empty.
		}

		return []MetricEntry{entry}
	})
}
