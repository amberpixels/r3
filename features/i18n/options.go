package i18n

import (
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// IDFunc extracts the primary key from an entity. The decorator stringifies
// it (fmt.Sprint) to match Translation.EntityID.
type IDFunc[T any, ID comparable] func(entity T) ID

// Options configures the behavior of a translations CRUD decorator.
type Options[T any, ID comparable] struct {
	// EntityType names this entity type in Translation rows. If empty, it is
	// derived from the struct type T (e.g. Location -> "locations").
	EntityType string

	// Fields lists the storage names of the translatable fields (db-tag
	// names, e.g. "title", "description"). Required; every name must resolve
	// to a string field on T.
	Fields []string

	// IDFunc extracts the primary key from an entity. Required.
	IDFunc IDFunc[T, ID]

	// ExcludeStale hides stale translations on reads, falling back to source text.
	// Default: stale translations are served (usually better than the wrong
	// language).
	ExcludeStale bool

	// SkipOverlay disables the read overlay: Get/List return source text untouched
	// while Update/Patch still mark translations stale. For editor/admin repos,
	// where the form shows and saves source text but source edits must still
	// invalidate translations - the safe answer to the read-modify-write hazard in
	// the package doc.
	SkipOverlay bool

	// DeleteWithEntity removes an entity's translations when it is deleted. Default
	// false: keeping them is the safe choice with soft-deleted, restorable entities.
	DeleteWithEntity bool

	// ErrorHandler is invoked when a best-effort side operation fails (overlay read,
	// staleness patch, delete cleanup). If nil, failures are logged via slog.
	// Reads/mutations themselves never fail over it.
	ErrorHandler func(error)
}

// Option is a functional option for configuring the decorator.
type Option[T any, ID comparable] func(*Options[T, ID])

// WithEntityType sets an explicit entity type name.
func WithEntityType[T any, ID comparable](name string) Option[T, ID] {
	return func(o *Options[T, ID]) { o.EntityType = name }
}

// WithFields declares which fields of T are translatable, by storage name
// (db-tag name or snake_cased Go name). Required.
func WithFields[T any, ID comparable](names ...string) Option[T, ID] {
	return func(o *Options[T, ID]) { o.Fields = append(o.Fields, names...) }
}

// WithIDFunc sets the function that extracts the primary key from an entity.
// Required.
func WithIDFunc[T any, ID comparable](fn IDFunc[T, ID]) Option[T, ID] {
	return func(o *Options[T, ID]) { o.IDFunc = fn }
}

// WithoutStale hides stale translations on reads (fall back to source text).
func WithoutStale[T any, ID comparable]() Option[T, ID] {
	return func(o *Options[T, ID]) { o.ExcludeStale = true }
}

// WithoutOverlay makes this a staleness-tracking-only decorator: reads pass
// through untranslated, writes still invalidate translations. For admin/editor
// repositories.
func WithoutOverlay[T any, ID comparable]() Option[T, ID] {
	return func(o *Options[T, ID]) { o.SkipOverlay = true }
}

// WithDeleteOnEntityDelete removes an entity's translations when the entity
// is deleted. Leave off for soft-deleted entities that may be restored.
func WithDeleteOnEntityDelete[T any, ID comparable]() Option[T, ID] {
	return func(o *Options[T, ID]) { o.DeleteWithEntity = true }
}

// WithErrorHandler sets a handler for best-effort side-operation failures,
// overriding the default slog logging.
func WithErrorHandler[T any, ID comparable](fn func(error)) Option[T, ID] {
	return func(o *Options[T, ID]) { o.ErrorHandler = fn }
}

// applyDefaults fills in zero-value options with sensible defaults.
func applyDefaults[T any, ID comparable](opts *Options[T, ID]) {
	if opts.EntityType == "" {
		opts.EntityType = deriveEntityType[T]()
	}
}

// deriveEntityType derives the entity type name from the struct type T
// (e.g. Location -> "locations"), matching features/history's derivation.
func deriveEntityType[T any]() string {
	return r3utils.ToSnakeCasePlural(typeName[T]())
}
