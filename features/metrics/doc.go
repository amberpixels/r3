// Package metrics provides domain-level analytics for r3 CRUD repositories.
//
// It works as a decorator around any r3.CRUD[T, ID] implementation, recording
// configurable metrics for every CRUD operation. The decorator is transparent:
// the wrapped repository still satisfies r3.CRUD[T, ID].
//
// "Everything is a R3po" -- metric records are themselves r3 entities,
// stored via any r3.CRUD[MetricRecord, string]. The metrics feature has zero
// knowledge of storage backends. You can use the same driver (SQL, GORM, MongoDB, etc.)
// that you use for your entities, or a completely different one.
//
// Key features:
//   - 10 built-in collectors covering common CRUD analytics (action counts, latency,
//     popularity, error rates, filter analysis, field changes, lifecycle, patch size)
//   - Extensible Collector[T, ID] interface for custom metrics
//   - Three-layer label system: core (auto), context, and entity labels
//   - Time bucketing for efficient aggregation (minutely to monthly)
//   - Aggregator with Count, Sum, Avg, TopN, TimeSeries, GroupBy
//   - AggregationPusher interface for server-side aggregation in capable backends
//   - Retention policies (age-based TTL, record count limits)
//   - Rollup/compaction of old fine-grained records into coarser summaries
//   - Async mode for zero-latency impact on the hot path
//   - r3.Actor context for automatic actor attribution
//
// # Aggregation
//
// The Aggregator performs in-memory aggregation by default. If the underlying
// store also implements AggregationPusher, the Aggregator automatically delegates
// to server-side aggregation for better performance at scale.
//
// # Retention and Rollup
//
// RetentionEnforcer deletes old metric records based on age (MaxAge) or count
// (MaxRecords). RollupExecutor compacts old fine-grained records into coarser
// time-bucket summaries, preserving label dimensions and annotating summaries
// with _rollup metadata labels.
//
// Both retention and rollup delete records one-by-one via the r3.CRUD Delete
// method. For large datasets this may be slow; a future BatchDelete interface
// could optimize bulk deletions for capable backends.
//
// The RollupExecutor compacts all old records for a given type regardless of
// their original bucket granularity. RollupPolicy.SourceBucket is recorded as
// provenance metadata only. To selectively compact only records of a specific
// granularity, filter them at the query level before rollup.
//
// RetentionEnforcer.Start and RollupExecutor run on-demand or via a ticker-based
// background loop. The background loop fires after the first interval elapses,
// not immediately on start.
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
