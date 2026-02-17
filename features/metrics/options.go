package metrics

// IDFunc extracts the primary key from an entity and returns it as a comparable value.
// Used by the decorator to populate RecordID on metric records and to fetch old
// entity state before updates/deletes.
type IDFunc[T any, ID comparable] func(entity T) ID

// Options configures the behavior of a metrics CRUD decorator.
type Options[T any, ID comparable] struct {
	// RecordType is the name used to identify this entity type in metric records.
	// If empty, it is derived automatically from the struct type T
	// (e.g. Order -> "orders", CampaignAdset -> "campaign_adsets").
	RecordType string

	// IDFunc extracts the primary key from an entity.
	// Required for entity-level metrics (popularity, per-entity counters).
	IDFunc IDFunc[T, ID]

	// Collectors defines what metrics to emit for CRUD operations.
	// Multiple collectors are composed — all are called on every operation.
	Collectors []Collector[T, ID]

	// Labelers extract dimension labels from entities.
	// Applied to every metric record where an entity is available.
	Labelers []Labeler[T]

	// ContextLabelers extract dimension labels from request context.
	// Applied to every metric record.
	ContextLabelers []ContextLabeler

	// BucketSize controls the time granularity for metric bucketing.
	// Default: BucketDaily.
	BucketSize BucketSize

	// Async when true records metrics in a background goroutine.
	// The CRUD operation returns immediately; metric recording errors are logged
	// but do not affect the CRUD result. Default: false (synchronous).
	Async bool

	// ErrorHandler is called when metric persistence fails.
	// If nil, errors are logged via slog.
	ErrorHandler func(error)
}

// Option is a functional option for configuring the metrics CRUD decorator.
type Option[T any, ID comparable] func(*Options[T, ID])

// WithRecordType sets an explicit record type name for metric records.
func WithRecordType[T any, ID comparable](name string) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.RecordType = name
	}
}

// WithIDFunc sets the function that extracts the primary key from an entity.
func WithIDFunc[T any, ID comparable](fn IDFunc[T, ID]) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.IDFunc = fn
	}
}

// WithCollectors sets the metric collectors.
func WithCollectors[T any, ID comparable](collectors ...Collector[T, ID]) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.Collectors = append(o.Collectors, collectors...)
	}
}

// WithLabelers sets entity-based label extractors.
func WithLabelers[T any, ID comparable](labelers ...Labeler[T]) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.Labelers = append(o.Labelers, labelers...)
	}
}

// WithContextLabelers sets context-based label extractors.
func WithContextLabelers[T any, ID comparable](labelers ...ContextLabeler) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.ContextLabelers = append(o.ContextLabelers, labelers...)
	}
}

// WithBucketSize sets the time bucketing granularity.
func WithBucketSize[T any, ID comparable](size BucketSize) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.BucketSize = size
	}
}

// WithAsync enables asynchronous metric recording.
func WithAsync[T any, ID comparable]() Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.Async = true
	}
}

// WithErrorHandler sets a custom error handler for metric persistence failures.
func WithErrorHandler[T any, ID comparable](fn func(error)) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.ErrorHandler = fn
	}
}

// applyDefaults fills in zero-value options with sensible defaults.
func applyDefaults[T any, ID comparable](opts *Options[T, ID]) {
	if opts.RecordType == "" {
		opts.RecordType = deriveRecordType[T]()
	}
	if opts.BucketSize == "" {
		opts.BucketSize = BucketDaily
	}
}
