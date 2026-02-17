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

// ListFilterCollector tracks which fields and operators users use in List queries.
// Emits one "crud.list.filter" entry per leaf filter in the query.
// Labels: {"field": "<name>", "operator": "<op>"}.
//
// This helps answer "what do users search for?" --
// e.g. 80% of queries filter by "status", so you need an index.
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

// walkFilters recursively traverses the filter tree and emits a metric entry for each leaf filter.
func walkFilters(filters r3.Filters, entries *[]MetricEntry) {
	for _, f := range filters {
		if f == nil {
			continue
		}

		// If this is a leaf filter (has a field), emit a metric
		if f.Field != nil {
			labels := Labels{"field": f.Field.String()}
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

		// Recurse into AND/OR groups
		if len(f.And) > 0 {
			walkFilters(f.And, entries)
		}
		if len(f.Or) > 0 {
			walkFilters(f.Or, entries)
		}
	}
}

// ListResultSizeCollector tracks the number of results returned by each List query.
// Emits one "crud.list.result_size" entry with value = len(results).
//
// Helps identify inefficient queries (too many results) or empty searches.
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

// ListTotalCountCollector tracks the total number of matching entities for each List query.
// Emits one "crud.list.total_count" entry with value = totalCount.
//
// Useful for tracking data volume growth over time.
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
