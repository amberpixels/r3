package r3

import "errors"

// ErrNotFound is returned by Get (and other single-record ops) when no record
// matches. Every backend normalizes its native "no rows"/"no documents" error to
// this sentinel - sql.ErrNoRows, gorm.ErrRecordNotFound, mongo.ErrNoDocuments,
// the file engine's internal not-found - so errors.Is detects a missing record
// identically across drivers.
var ErrNotFound = errors.New("r3: record not found")

// ErrAggregateNotSupported is returned by [AggregateOf] when the repository
// (or a decorator in its chain) does not implement [Aggregator].
var ErrAggregateNotSupported = errors.New("r3: aggregate not supported")

// ErrUpsertNotSupported is returned by [UpsertOf] when the repository (or a
// decorator in its chain) does not implement [Upserter].
var ErrUpsertNotSupported = errors.New("r3: upsert not supported")

// ErrBulkPatchNotSupported is returned by [PatchWhereOf] when the repository (or
// a decorator in its chain) does not implement [BulkPatcher].
var ErrBulkPatchNotSupported = errors.New("r3: bulk patch not supported")

// ErrUnknownCodec wraps a panic from [SchemaOf] when a r3:"...,codec:<name>" tag
// names an unregistered codec. A tag typo is a deterministic developer error, so
// it fails loudly at derivation rather than silently leaving the field un-encoded.
var ErrUnknownCodec = errors.New("r3: unknown codec")

// ErrCodecNotSupported wraps a construction-time panic when an entity declares a
// codec that the target backend does not yet apply. Silently ignoring the codec
// would store the un-encoded value - data corruption, not graceful degradation -
// so such backends panic here instead (see [RequireCodecSupport]).
var ErrCodecNotSupported = errors.New("r3: value codec not supported by this backend")

// ErrInvalidAggregate is returned when an aggregate query is structurally
// invalid: no aggregates declared, a missing/duplicate/invalid alias, an
// aggregate function that requires a field called without one, an alias
// colliding with a group field, a Having filter referencing an undeclared
// alias, or SUM/AVG over an attribute the schema knows to be non-numeric.
var ErrInvalidAggregate = errors.New("r3: invalid aggregate query")

// Schema validation errors. Schema.ValidateQuery wraps the offending field name
// (fmt.Errorf("%w: %q", err, name)) so a consumer can surface a 400-class message
// instead of leaking a backend driver error (a 500) once SQL is built.
var (
	// ErrUnknownField is returned when a referenced field is not declared by the schema.
	ErrUnknownField = errors.New("unknown field")
	// ErrFieldNotFilterable is returned when a non-filterable field appears in Query.Filters.
	ErrFieldNotFilterable = errors.New("field is not filterable")
	// ErrFieldNotSortable is returned when a non-sortable field appears in Query.Sorts.
	ErrFieldNotSortable = errors.New("field is not sortable")
	// ErrFieldNotQueryable is returned when a non-queryable field appears in Query.Fields.
	ErrFieldNotQueryable = errors.New("field is not queryable")
)
