package metrics

import "context"

// Labeler extracts dimension labels from an entity; composed labelers' outputs
// are merged into every metric record.
//
//	orderTypeLabeler := func(o Order) metrics.Labels {
//	    return metrics.Labels{"order_type": o.Type}
//	}
type Labeler[T any] func(entity T) Labels

// ContextLabeler extracts dimension labels from request context, for labels that
// come from the HTTP layer rather than the entity.
type ContextLabeler func(ctx context.Context) Labels
