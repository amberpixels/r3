package metrics

// IDFunc extracts an entity's primary key. The decorator uses it to populate
// RecordID and to fetch old entity state before updates/deletes.
type IDFunc[T any, ID comparable] func(entity T) ID

// Options configures a metrics CRUD decorator.
type Options[T any, ID comparable] struct {
	// RecordType names this entity type in metric records. If empty, derived from
	// T as snake_case plural (e.g. Order -> "orders").
	RecordType string

	// IDFunc extracts the primary key; required for entity-level metrics.
	IDFunc IDFunc[T, ID]

	// Collectors are the metrics emitted per operation; all run on every op.
	Collectors []Collector[T, ID]

	// Labelers extract labels from entities, applied wherever an entity is available.
	Labelers []Labeler[T]

	// ContextLabelers extract labels from request context, applied to every record.
	ContextLabelers []ContextLabeler

	// BucketSize is the time bucketing granularity. Default: BucketDaily.
	BucketSize BucketSize

	// Async records metrics in a background goroutine: the CRUD op returns
	// immediately and recording errors are logged, not surfaced. Default: false.
	Async bool

	// ErrorHandler is called on persistence failure; if nil, errors go to slog.
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
