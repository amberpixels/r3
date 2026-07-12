package metrics

import (
	"context"
	"strconv"

	"github.com/amberpixels/r3"
)

// Metric names for list-related collectors.
const (
	// MetricListFilter tracks which fields users filter by in List queries.
	MetricListFilter = "crud.list.filter"

	// MetricListResultSize tracks how many results each List query returns.
	MetricListResultSize = "crud.list.result_size"

	// MetricListTotalCount tracks the total matching count from List queries (before pagination).
	MetricListTotalCount = "crud.list.total_count"
)

// ListFilterCollector emits one "crud.list.filter" entry per leaf filter,
// labeled {"field": "<name>", "operator": "<op>"} - answering "what do users
// filter by?".
func ListFilterCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil || opCtx.Operation != OpList {
			return nil
		}

		filters := opCtx.Query.Filters
		if len(filters) == 0 {
			return nil
		}

		var entries []MetricEntry
		walkFilters(filters, &entries)
		return entries
	})
}

// walkFilters emits a metric entry for each leaf filter in the tree.
func walkFilters(filters r3.Filters, entries *[]MetricEntry) {
	for _, f := range filters {
		if f == nil {
			continue
		}

		// A leaf filter carries a field.
		if f.Field != nil {
			labels := Labels{labelField: f.Field.String()}
			op := f.Operator
			if op != r3.OperatorUnspecified {
				labels["operator"] = op.String()
			}
			*entries = append(*entries, MetricEntry{
				MetricName: MetricListFilter,
				Value:      1,
				Labels:     labels,
			})
		}

		if len(f.And) > 0 {
			walkFilters(f.And, entries)
		}
		if len(f.Or) > 0 {
			walkFilters(f.Or, entries)
		}
	}
}

// ListResultSizeCollector emits one "crud.list.result_size" entry per List,
// valued as the number of results returned.
func ListResultSizeCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil || opCtx.Operation != OpList {
			return nil
		}

		return []MetricEntry{{
			MetricName: MetricListResultSize,
			Value:      float64(len(opCtx.Entities)),
			Labels: Labels{
				"result_count": strconv.Itoa(len(opCtx.Entities)),
				"total_count":  strconv.FormatInt(opCtx.TotalCount, 10),
			},
		}}
	})
}

// ListTotalCountCollector emits one "crud.list.total_count" entry per List,
// valued as the total matching count (before pagination) - tracks data growth.
func ListTotalCountCollector[T any, ID comparable]() Collector[T, ID] {
	return CollectorFunc[T, ID](func(_ context.Context, opCtx OperationContext[T, ID]) []MetricEntry {
		if opCtx.Err != nil || opCtx.Operation != OpList {
			return nil
		}

		return []MetricEntry{{
			MetricName: MetricListTotalCount,
			Value:      float64(opCtx.TotalCount),
		}}
	})
}
