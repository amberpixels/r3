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

// MergeWith merges given GetParams with some other GetParams.
func (q Query) MergeWith(other Query) Query {
	result := q.Clone()

	result.Fields = result.Fields.MergeWith(other.Fields)
	result.Filters = result.Filters.MergeWith(other.Filters)
	result.Sorts = result.Sorts.MergeWith(other.Sorts)
	result.Preloads = result.Preloads.MergeWith(other.Preloads)

	// For pagination merging
	if other.Pagination != nil {
		if result.Pagination != nil {
			result.Pagination = result.Pagination.MergeWith(other.Pagination)
		} else {
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
