package metrics

import (
	"context"
	"fmt"
)

// MetricCRUDAction is the metric name for the universal CRUD action counter.
// Every CRUD operation increments this metric by 1.
// The "operation" core label distinguishes create/get/list/update/patch/delete.
const MetricCRUDAction = "crud.action"

// CRUDActionCollector counts every CRUD operation.
// Emits one "crud.action" entry with value 1 on every successful operation.
//
// The operation type is already included as a core label ("operation")
// by the decorator, so this collector does not add it.
//
// RecordID is set for entity-level operations (create/get/update/patch/delete)
// and empty for list operations.
func CRUDActionCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		// Only fire on success
		if opCtx.Err != nil {
			return nil
		}

		entry := MetricEntry{
			MetricName: MetricCRUDAction,
			Value:      1,
		}

		// Set RecordID for entity-level operations
		switch opCtx.Operation {
		case OpCreate, OpGet, OpUpdate, OpPatch, OpDelete:
			entry.RecordID = fmt.Sprint(opCtx.EntityID)
		case OpList, OpCount:
			// List and Count are type-level; RecordID stays empty.
		}

		return []MetricEntry{entry}
	})
}
