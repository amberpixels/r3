package metrics

import (
	"context"
	"fmt"
)

// MetricEntityPopularity is the metric name for entity access scoring.
// Incremented when an entity is accessed via Get or appears in List results.
const MetricEntityPopularity = "entity.popularity"

// PopularityCollector emits "entity.popularity" (value 1) per entity accessed:
// on Get one entry labeled {"source": "get"}, on List one per result labeled
// {"source": "list"}. Needs IDFunc set so entity IDs are available; on List,
// entity labelers apply per-entity, overriding the decorator's single-entity merge.
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
