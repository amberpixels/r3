package r3

import (
	"github.com/amberpixels/k1/maybe"
)

// Query wraps everything from r3: Pagination, Fields, Filters, Sorts, Preloads.
type Query struct {
	Pagination *PaginationSpec

	// Cursor enables keyset/cursor-based pagination as an alternative to offset-based.
	// When set, Cursor takes precedence over Pagination. The two are mutually exclusive.
	Cursor *CursorSpec

	Fields   Fields   // Specific fields to retrieve.
	Filters  Filters  // []*FilterSpec
	Sorts    Sorts    // []*SortSpec
	Preloads Preloads // []*PreloadSpec

	// IncludeTrashed when true will still return trashed (soft-deleted) records.
	IncludeTrashed maybe.Bool
}

// NewQuery returns an empty Query.
func NewQuery() Query { return Query{} }

// DefaultQuery returns the default Query (with reasonable params).
func DefaultQuery() Query { q := NewQuery(); q.Pagination = DefaultPagination(); return q }

// MergeWith merges this query with another, returning a new Query (no mutation).
//
// Fields, Filters, and Preloads accumulate (the union of both). Sorts and
// Pagination OVERRIDE: when other specifies them, they replace the inherited
// values rather than stacking under them. This makes other the higher-precedence
// layer — typically a per-call query merged over a repo's defaults.
func (q Query) MergeWith(other Query) Query {
	result := q.Clone()

	result.Fields = result.Fields.MergeWith(other.Fields)
	result.Filters = result.Filters.MergeWith(other.Filters)
	result.Preloads = result.Preloads.MergeWith(other.Preloads)

	// Sorts OVERRIDE rather than accumulate: an explicit sort from the
	// higher-precedence query REPLACES the inherited (default) ordering. Appending
	// — as Fields/Filters/Preloads do — would keep the default sort as the primary
	// key and silently demote the requested sort to a tie-breaker (e.g. a default
	// "created_at DESC" would dominate a requested "price ASC"). This mirrors the
	// Pagination override below. A query with no sorts inherits the default.
	if len(other.Sorts) > 0 {
		result.Sorts = other.Sorts.Clone()
	}

	// For pagination merging.
	//
	// A non-nil but non-paginated spec (r3.Unpaginated / r3.NoPagination) is an
	// EXPLICIT "return everything" — it must clear any inherited default page
	// size, not be swallowed by a field-wise merge (an empty spec has no Some()
	// fields, so MergeWith would otherwise keep the default's size).
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

	// For cursor merging
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
	clone.IncludeTrashed = q.IncludeTrashed
	return clone
}
