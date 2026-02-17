package metrics

import "errors"

// Sentinel errors for metrics operations.
var (
	// ErrNoMetrics is returned when querying metrics for an entity with no recorded data.
	ErrNoMetrics = errors.New("r3metrics: no metrics found")

	// ErrIDFuncRequired is returned when a collector requires IDFunc but it's not set.
	ErrIDFuncRequired = errors.New("r3metrics: IDFunc is required for entity-level metrics; use WithIDFunc option")
)
