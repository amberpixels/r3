package r3

import "errors"

// ErrNotFound is returned by Get (and other single-record operations) when no
// record matches the requested ID.
//
// Every backend normalizes its native "no rows" / "no documents" error to this
// sentinel — database/sql's sql.ErrNoRows, GORM's gorm.ErrRecordNotFound,
// MongoDB's mongo.ErrNoDocuments, and the file engine's internal not-found
// error all surface as r3.ErrNotFound. This lets business code detect a missing
// record identically regardless of the concrete driver:
//
//	user, err := repo.Get(ctx, id)
//	if errors.Is(err, r3.ErrNotFound) {
//	    // respond 404, etc.
//	}
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

// ErrInvalidAggregate is returned when an aggregate query is structurally
// invalid: no aggregates declared, a missing/duplicate/invalid alias, an
// aggregate function that requires a field called without one, an alias
// colliding with a group field, a Having filter referencing an undeclared
// alias, or SUM/AVG over an attribute the schema knows to be non-numeric.
var ErrInvalidAggregate = errors.New("r3: invalid aggregate query")

// Schema validation errors. Schema.ValidateQuery wraps the offending field name
// (fmt.Errorf("%w: %q", err, name)) so a consumer can surface a useful 400-class
// message instead of leaking a backend driver error (which would otherwise be a 500).
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
