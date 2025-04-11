package r3atoms

import "github.com/amberpixels/r3/internal/option"

const (
	// LimitDefault is a GLOBAL per-package default if no pagination was specified.
	LimitDefault = 50
)

// Pagination represents the pagination parameters for a Repo request.
type Pagination struct {
	Limit  option.Int `json:"limit"`
	Offset option.Int `json:"offset"`
}

// NewPagination returns a new Pagination with the given limit and offset.
func NewPagination(limit int, offset ...int) Pagination {
	p := Pagination{Limit: option.Some(limit)}
	if len(offset) > 0 {
		p.Offset = option.Some(offset[0])
	}

	return p
}

// NoLimitPagination returns a new Pagination with no limit and offset
func NoLimitPagination() Pagination {
	return Pagination{}
}

func DefaultPagination() Pagination { return NewPagination(LimitDefault) }

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

func (p Pagination) Clone() Pagination { return Pagination{Limit: p.Limit, Offset: p.Offset} }

// MergeWith overrides pagination with some new values given
func (p Pagination) MergeWith(other Pagination) Pagination {
	result := p.Clone()

	p.Limit = other.Limit
	p.Offset = other.Offset

	return result
}

func (p Pagination) IsPaginated() bool {
	if p.Limit.IsZero() {
		return !p.Offset.IsZero()
	}

	return true
}
