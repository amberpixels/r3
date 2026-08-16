package r3

import (
	"fmt"
	"slices"
	"strings"

	"github.com/amberpixels/k1/maybe"
)

// Query is the immutable read model: filters, sorts, projection, pagination, and
// (for [Aggregator]) grouping. Compose with [Query.MergeWith]; never mutate a
// shared value.
type Query struct {
	Pagination *PaginationSpec

	// Cursor enables keyset/cursor pagination instead of offset-based. When set it
	// takes precedence over Pagination; the two are mutually exclusive.
	Cursor *CursorSpec

	Filters  Filters  // []*FilterSpec
	Sorts    Sorts    // []*SortSpec
	Preloads Preloads // []*PreloadSpec

	// Fields is the additive projection: return only these. Build it with
	// [Include]. The primary key is always returned whether or not Fields names
	// it - an entity without its identity cannot be patched, deleted, or linked.
	//
	// Mutually exclusive with [Query.ExcludeFields], which is its mirror image;
	// see there for the shared caveats.
	Fields Fields

	// ExcludeFields is the subtractive projection: return every queryable field
	// EXCEPT these. It answers "everything but the fat blob", which an additive
	// [Query.Fields] list can only express by enumerating the rest - and by
	// silently dropping any column added later. Build it with [Exclude].
	//
	// Mutually exclusive with Fields: setting both is [ErrProjectionConflict],
	// since a projection cannot be additive and subtractive at once (MongoDB
	// rejects the mixed form outright). [Query.MergeWith] is the one exception -
	// there the higher-precedence layer picks the form. The primary key is never
	// excluded, the mirror of Fields always including it.
	//
	// A projected entity is NOT safe to write back. Update and Upsert persist
	// every mutable field of the value they are given, so an entity read without
	// a column and then passed to Update stores that column's zero value. Read
	// unprojected (or use Patch, which writes only the named fields) whenever the
	// value is going back to the store. The same caution applies to Fields.
	ExcludeFields Fields

	// GroupBy, Buckets, Aggregates, and Having describe an aggregation honored
	// only by Aggregate (see [Aggregator]); Get/List/Count ignore them, as Count
	// ignores pagination. GroupBy names the plain grouping fields (empty = one
	// whole-set row when Buckets is also empty), Buckets adds derived time-bucket
	// group keys (see [BucketSpec]), Aggregates the computed values (required for
	// Aggregate), and Having filters grouped rows by aggregate alias, group field,
	// or bucket alias.
	GroupBy    Fields
	Buckets    Buckets
	Aggregates Aggregates
	Having     Filters

	// IncludeTrashed, when true, also returns soft-deleted records.
	IncludeTrashed maybe.Bool
}

// NewQuery returns an empty Query.
func NewQuery() Query { return Query{} }

// DefaultQuery returns a Query with the default pagination applied.
func DefaultQuery() Query { q := NewQuery(); q.Pagination = DefaultPagination(); return q }

// MergeWith returns a new Query combining q with other (no mutation). Fields,
// ExcludeFields, Filters, and Preloads accumulate (union). Sorts and Pagination
// OVERRIDE - other is the higher-precedence layer, typically a per-call query
// over a repo's defaults.
func (q Query) MergeWith(other Query) Query {
	result := q.Clone()

	result.Fields = result.Fields.MergeWith(other.Fields)
	result.ExcludeFields = result.ExcludeFields.MergeWith(other.ExcludeFields)
	result.Filters = result.Filters.MergeWith(other.Filters)
	result.Preloads = result.Preloads.MergeWith(other.Preloads)

	// The two projection forms union within a form but cannot coexist across
	// them, so when the layers disagree on the form, other wins - the same
	// higher-precedence rule Sorts and Pagination follow. Without it a repo
	// default naming Fields would turn every per-call Exclude into an
	// unavoidable [ErrProjectionConflict]. Other naming both is the caller's own
	// conflict, left intact so validation reports it.
	switch {
	case len(other.Fields) > 0 && len(other.ExcludeFields) == 0:
		result.ExcludeFields = nil
	case len(other.ExcludeFields) > 0 && len(other.Fields) == 0:
		result.Fields = nil
	}

	// Sorts OVERRIDE rather than accumulate: appending would keep the default sort
	// as the primary key and demote the requested one to a tie-breaker (a default
	// "created_at DESC" would dominate a requested "price ASC"). No sorts inherits
	// the default.
	if len(other.Sorts) > 0 {
		result.Sorts = other.Sorts.Clone()
	}

	// The aggregation shape (GroupBy + Buckets + Aggregates + Having) OVERRIDES as
	// a unit: it defines what the result rows ARE, so stacking a default under a
	// requested shape would corrupt both.
	if len(other.GroupBy) > 0 || len(other.Buckets) > 0 || len(other.Aggregates) > 0 || len(other.Having) > 0 {
		result.GroupBy = other.GroupBy.Clone()
		result.Buckets = other.Buckets.Clone()
		result.Aggregates = other.Aggregates.Clone()
		result.Having = other.Having.Clone()
	}

	// A non-nil but non-paginated spec ([Unpaginated]/[NoPagination]) is an
	// EXPLICIT "return everything": it must CLEAR any inherited default page size,
	// not be swallowed by the field-wise merge (an empty spec has no Some() fields,
	// so MergeWith would otherwise keep the default's size).
	if other.Pagination != nil {
		switch {
		case !other.Pagination.IsPaginated():
			result.Pagination = other.Pagination.Clone()
		case result.Pagination != nil:
			result.Pagination = result.Pagination.MergeWith(other.Pagination)
		default:
			result.Pagination = other.Pagination.Clone()
		}
	}

	if other.Cursor != nil {
		if result.Cursor != nil {
			result.Cursor = result.Cursor.MergeWith(other.Cursor)
		} else {
			result.Cursor = other.Cursor.Clone()
		}
	}

	if other.IncludeTrashed.Some() {
		result.IncludeTrashed = other.IncludeTrashed
	}

	return result
}

// ValidateProjection checks the structural rule both projection forms share: a
// query names fields to keep or fields to drop, never both. Engines call it
// before lowering a projection, so the conflict surfaces as a typed error rather
// than as a backend complaint about a mixed projection document.
//
// It knows nothing about the model, so it cannot catch a name that matches no
// field; engines pair it with their own field lookup (see
// [Schema.ValidateQuery]).
func (q Query) ValidateProjection() error {
	if len(q.Fields) > 0 && len(q.ExcludeFields) > 0 {
		return ErrProjectionConflict
	}
	return nil
}

// ValidateProjectionAgainst is [Query.ValidateProjection] plus the check that
// every excluded field is one the backend actually stores, given its field or
// column names. Engines whose read path does not run [Schema.ValidateQuery] call
// this with their own reflected names, which is the authority on what the store
// holds (a Mongo model names its fields via `bson` tags, a file-backed one via
// `json`, neither of which [SchemaOf] reads).
//
// An excluded name matching nothing drops nothing, so without this a typo passes
// silently and hands back the very field the caller asked to leave behind - the
// one mistake the subtractive form exists to prevent. Names in Fields are not
// checked: an unrecognized one there is visibly absent from the result.
// Dotted relation paths are skipped, as elsewhere in the query validators.
func (q Query) ValidateProjectionAgainst(known []string) error {
	if err := q.ValidateProjection(); err != nil {
		return err
	}
	for _, name := range FieldsToStrings(q.ExcludeFields) {
		if name == "" || strings.Contains(name, ".") {
			continue
		}
		if !slices.Contains(known, name) {
			return fmt.Errorf("%w: %q", ErrUnknownField, name)
		}
	}
	return nil
}

// Clone clones the query.
func (q Query) Clone() Query {
	var clone Query
	clone.Pagination = q.Pagination.Clone()
	clone.Cursor = q.Cursor.Clone()
	clone.Fields = q.Fields.Clone()
	clone.ExcludeFields = q.ExcludeFields.Clone()
	clone.Filters = q.Filters.Clone()
	clone.Sorts = q.Sorts.Clone()
	clone.Preloads = q.Preloads.Clone()
	clone.GroupBy = q.GroupBy.Clone()
	clone.Buckets = q.Buckets.Clone()
	clone.Aggregates = q.Aggregates.Clone()
	clone.Having = q.Having.Clone()
	clone.IncludeTrashed = q.IncludeTrashed
	return clone
}
