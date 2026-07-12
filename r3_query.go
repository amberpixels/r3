package r3

import (
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

	Fields   Fields   // Specific fields to retrieve.
	Filters  Filters  // []*FilterSpec
	Sorts    Sorts    // []*SortSpec
	Preloads Preloads // []*PreloadSpec

	// GroupBy, Aggregates, and Having describe an aggregation honored only by
	// Aggregate (see [Aggregator]); Get/List/Count ignore them, as Count ignores
	// pagination. GroupBy names the grouping fields (empty = one whole-set row),
	// Aggregates the computed values (required for Aggregate), and Having filters
	// grouped rows by aggregate alias or group field.
	GroupBy    Fields
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
// Filters, and Preloads accumulate (union). Sorts and Pagination OVERRIDE - other
// is the higher-precedence layer, typically a per-call query over a repo's defaults.
func (q Query) MergeWith(other Query) Query {
	result := q.Clone()

	result.Fields = result.Fields.MergeWith(other.Fields)
	result.Filters = result.Filters.MergeWith(other.Filters)
	result.Preloads = result.Preloads.MergeWith(other.Preloads)

	// Sorts OVERRIDE rather than accumulate: appending would keep the default sort
	// as the primary key and demote the requested one to a tie-breaker (a default
	// "created_at DESC" would dominate a requested "price ASC"). No sorts inherits
	// the default.
	if len(other.Sorts) > 0 {
		result.Sorts = other.Sorts.Clone()
	}

	// The aggregation shape (GroupBy + Aggregates + Having) OVERRIDES as a unit: it
	// defines what the result rows ARE, so stacking a default under a requested
	// shape would corrupt both.
	if len(other.GroupBy) > 0 || len(other.Aggregates) > 0 || len(other.Having) > 0 {
		result.GroupBy = other.GroupBy.Clone()
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

// Clone clones the query.
func (q Query) Clone() Query {
	var clone Query
	clone.Pagination = q.Pagination.Clone()
	clone.Cursor = q.Cursor.Clone()
	clone.Fields = q.Fields.Clone()
	clone.Filters = q.Filters.Clone()
	clone.Sorts = q.Sorts.Clone()
	clone.Preloads = q.Preloads.Clone()
	clone.GroupBy = q.GroupBy.Clone()
	clone.Aggregates = q.Aggregates.Clone()
	clone.Having = q.Having.Clone()
	clone.IncludeTrashed = q.IncludeTrashed
	return clone
}
