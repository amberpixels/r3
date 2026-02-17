package metrics

import "context"

// Labeler extracts dimension labels from an entity.
// Multiple labelers can be composed — their outputs are merged into every metric record.
//
// Example:
//
//	orderTypeLabeler := func(o Order) metrics.Labels {
//	    return metrics.Labels{"order_type": o.Type}
//	}
type Labeler[T any] func(entity T) Labels

// ContextLabeler extracts dimension labels from request context.
// Useful for labels that come from the HTTP layer, not the entity itself.
//
// Example:
//
//	func sourceLabeler(ctx context.Context) metrics.Labels {
//	    return metrics.Labels{"source": middleware.GetSource(ctx)}
//	}
type ContextLabeler func(ctx context.Context) Labels
