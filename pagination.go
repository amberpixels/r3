package r3

import (
	"github.com/amberpixels/k1/maybe"
)

// TODO: Refactor the r3 core Pagination to be PageNumber / PageSize style
//       Page count starts from 1 (no page, or page = 0 is considered page = 1, but with a warn log - something is badly configured)
//
// TODO: also, we need to be consistent with Fields and Filters - so we have an interface and a default implement (PaginationSpec)
//       Pagination interface will also contain Stringer and Cloner as usually

const (
	// LimitDefault is a GLOBAL per-package default if no pagination was specified.
	LimitDefault = 50
)

// Pagination represents the pagination parameters for a Repo request.
type Pagination struct {
	Limit  maybe.Int `json:"limit"`
	Offset maybe.Int `json:"offset"`
}

// NewPagination returns a new Pagination with the given limit and offset.
func NewPagination(limit int, offset ...int) Pagination {
	p := Pagination{Limit: maybe.Some(limit)}
	if len(offset) > 0 {
		p.Offset = maybe.Some(offset[0])
	}

	return p
}

// NoLimitPagination returns a new Pagination with no limit and offset.
func NoLimitPagination() Pagination { return Pagination{} }

// DefaultPagination returns Pagination struct with defaults.
func DefaultPagination() Pagination { return NewPagination(LimitDefault) }

// TODO: Refactor to fit Visitor pattern same as Fields & Filters
func (p Pagination) GetDialectLimitOffset() (int, int) {
	var limit int
	if p.Limit.Some() {
		limit = p.Limit.Unwrap()
	}

	var offset int
	if p.Offset.Some() {
		offset = p.Offset.Unwrap()
	}

	return limit, offset
}

// Clone clones the Pagination structs.
func (p Pagination) Clone() Pagination { return Pagination{Limit: p.Limit, Offset: p.Offset} }

// MergeWith overrides pagination with some new values given.
func (p Pagination) MergeWith(other Pagination) Pagination {
	result := p.Clone()

	if other.Limit.Some() {
		result.Limit = other.Limit
	}
	if other.Offset.Some() {
		result.Offset = other.Offset
	}

	return result
}

// IsPaginated returns true if pagination is set.
func (p Pagination) IsPaginated() bool {
	if p.Limit.IsZero() {
		return !p.Offset.IsZero()
	}

	return true
}
