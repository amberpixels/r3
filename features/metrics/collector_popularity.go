package metrics

import (
	"context"
	"fmt"
)

// MetricEntityPopularity is the metric name for entity access scoring.
// Incremented when an entity is accessed via Get or appears in List results.
const MetricEntityPopularity = "entity.popularity"

// PopularityCollector tracks how often individual entities are accessed.
// Emits "entity.popularity" with value 1 for each entity accessed:
//   - On Get: one entry for the fetched entity, labeled {"source": "get"}
//   - On List: one entry per result entity, labeled {"source": "list"}
//
// Requires IDFunc to be set on the decorator options so entity IDs are available.
// For List operations, entity labelers are applied per-entity (overriding
// the decorator's single-entity label merge).
func PopularityCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil {
			return nil
		}

		switch opCtx.Operation {
		case OpGet:
			return []MetricEntry{{
				MetricName: MetricEntityPopularity,
				Value:      1,
				RecordID:   fmt.Sprint(opCtx.EntityID),
				Labels:     Labels{"source": "get"},
			}}

		case OpList:
			// One entry per entity in the result set.
			entries := make([]MetricEntry, 0, len(opCtx.Entities))
			for range opCtx.Entities {
				entries = append(entries, MetricEntry{
					MetricName: MetricEntityPopularity,
					Value:      1,
					Labels:     Labels{"source": "list"},
				})
			}
			return entries

		case OpCreate, OpCount, OpAggregate, OpUpdate, OpPatch, OpDelete, OpUpsert, OpPatchWhere:
			return nil
		}

		return nil
	})
}
