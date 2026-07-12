// Package metrics decorates any r3.CRUD[T, ID] with domain-level analytics,
// recording configurable metrics for every CRUD operation. Metric records are
// themselves r3 entities, persisted via any r3.CRUD[MetricRecord, string] - the
// same or a different backend than the wrapped entity uses.
//
// Ten built-in collectors cover common CRUD analytics (action counts, latency,
// popularity, error rates, filter analysis, field changes, lifecycle, patch
// size); the Collector[T, ID] interface adds custom ones. Labels come in three
// layers - core (auto), context, entity - and records are time-bucketed
// (minutely to monthly) for cheap aggregation. Async mode keeps the hot path
// latency-free; r3.Actor is picked up from context for attribution.
//
// # Aggregation
//
// The Aggregator aggregates in-memory, or delegates to server-side aggregation
// when the store also implements AggregationPusher.
//
// # Retention and Rollup
//
// RetentionEnforcer deletes old records by age (MaxAge) or count (MaxRecords).
// RollupExecutor compacts old fine-grained records into coarser time-bucket
// summaries, preserving label dimensions and tagging summaries with _rollup
// metadata labels.
//
// Both delete one record at a time via r3.CRUD Delete (slow at scale; a future
// BatchDelete could optimize this). RollupExecutor compacts all old records for
// a type regardless of source granularity; RollupPolicy.SourceBucket is
// provenance metadata only - filter at the query level to target one
// granularity. The ticker in RetentionEnforcer.Start (and any rollup loop)
// first fires after one interval elapses, not immediately.
//
// # Usage
//
//	metricsStore := r3pgx.NewPgxCRUD[metrics.MetricRecord, string](db)
//
//	repo := metrics.WithMetrics[Order, int64](
//	    innerRepo,
//	    metricsStore,
//	    metrics.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
//	    metrics.WithCollectors[Order, int64](
//	        metrics.CRUDActionCollector[Order, int64](),
//	        metrics.PopularityCollector[Order, int64](),
//	        metrics.LatencyCollector[Order, int64](),
//	    ),
//	    metrics.WithLabelers[Order, int64](
//	        func(o Order) metrics.Labels { return metrics.Labels{"order_type": o.Type} },
//	    ),
//	    metrics.WithAsync[Order, int64](),
//	)
//
//	// Query metrics:
//	agg := metrics.NewAggregator(metricsStore)
//	count, _ := agg.Count(ctx, "orders", "crud.action", metrics.LastMonth(),
//	    metrics.WithLabel("operation", "create"))
//	top, _ := agg.TopN(ctx, "orders", "entity.popularity", metrics.LastWeek(), 10)
package metrics
