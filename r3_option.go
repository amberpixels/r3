package r3

// Option is a functional option for configuring R3 repositories.
// Pass options to engine/driver constructors to customize behavior.
type Option func(*Options)

// Options holds the resolved configuration for a repository.
// Engine and driver constructors call [ResolveOptions] to apply
// functional options and get the final values.
type Options struct {
	// Config is the framework-level configuration.
	Config Config

	// Relations holds relations declared explicitly (by table + column names)
	// via [WithRelations], to supplement or override the relations reflected
	// from `r3:"rel:..."` struct tags. See [RelationSpec].
	Relations []RelationSpec
}

// DefaultOptions returns Options initialized with sensible defaults.
func DefaultOptions() Options {
	return Options{
		Config: DefaultConfig(),
	}
}

// ResolveOptions applies functional options to the default [Options]
// and returns the result.
func ResolveOptions(opts ...Option) Options {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithConfig sets the R3 framework-level configuration.
func WithConfig(cfg Config) Option {
	return func(o *Options) {
		o.Config = cfg
	}
}

// WithRelations registers relations declared explicitly by table and column
// names (see [RelationSpec]), letting a repository resolve relations to tables
// it does not import as Go types. Declared relations supplement the relations
// reflected from `r3:"rel:..."` tags; a declared relation whose name matches a
// reflected one takes precedence. They are resolvable through [Has]/[HasNo]
// filters and [AggregateThroughRelation].
//
//	repo := r3gorm.NewGormCRUD[Photo, int64](db,
//		r3.WithRelations(
//			r3.ManyToManyRelation("locations",
//				"photo_locations", "photo_id", "location_id", "locations"),
//		),
//	)
//
// Relation support is currently implemented by the GORM driver only.
func WithRelations(rels ...RelationSpec) Option {
	return func(o *Options) {
		o.Relations = append(o.Relations, rels...)
	}
}
