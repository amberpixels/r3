package r3

import (
	"fmt"

	"github.com/amberpixels/k1/maybe"
)

const (
	// PageSizeDefault is the package-wide default page size when none is specified.
	PageSizeDefault = 100
)

// PaginationSpec is the core pagination type using PageNum/PageSize.
type PaginationSpec struct {
	// PageNum is 1-indexed page number (1 = first page).
	PageNum maybe.Int
	// PageSize is the number of items per page.
	PageSize maybe.Int
	// Offset, when set, is the raw 0-based row offset - an alternative to PageNum
	// for consumers that natively think in offsets (REST ?offset=, infinite-scroll
	// loaders). Unlike a page number it can express a starting row that is not a
	// multiple of PageSize. When set it takes precedence over PageNum in
	// ToLimitOffset and is carried to the driver verbatim (no page rounding).
	// PageNum and Offset are mutually exclusive position encodings (see MergeWith).
	Offset maybe.Int
}

// NewPaginationSpec creates a new PaginationSpec with given page number and page size.
func NewPaginationSpec(pageNum int, pageSize ...int) *PaginationSpec {
	p := &PaginationSpec{PageNum: maybe.Some(pageNum)}
	if len(pageSize) > 0 {
		p.PageSize = maybe.Some(pageSize[0])
	}
	return p
}

// NewPaginationSpecWithSize creates a new PaginationSpec with only page size specified.
func NewPaginationSpecWithSize(pageSize int) *PaginationSpec {
	return &PaginationSpec{PageSize: maybe.Some(pageSize)}
}

// NewOffsetPagination creates a PaginationSpec from a raw 0-based row offset (and
// an optional limit), for consumers that paginate by offset rather than page
// number (REST ?offset=, infinite-scroll loaders). Unlike a page number, an
// offset can start at a row that is not a multiple of the limit:
// NewOffsetPagination(100, 30) fetches exactly rows 100-129. The offset is
// carried to the driver verbatim, so no page rounding can silently duplicate or
// skip rows. A negative offset is treated as 0; an omitted limit defaults to
// PageSizeDefault (mirrors NewPaginationSpec).
func NewOffsetPagination(offset int, limit ...int) *PaginationSpec {
	p := &PaginationSpec{Offset: maybe.Some(offset)}
	if len(limit) > 0 {
		p.PageSize = maybe.Some(limit[0])
	}
	return p
}

// NoPagination returns a PaginationSpec with no pagination limits.
func NoPagination() *PaginationSpec {
	return &PaginationSpec{}
}

// Unpaginated disables the default page-size cap so List returns every matching
// record - otherwise a "give me everything" caller silently gets a truncated
// slice. It overrides any configured default page size on merge:
//
//	all, total, err := repo.List(ctx, r3.Query{Pagination: r3.Unpaginated()})
//
// To make a whole repo unpaginated by default, set Config.Defaults.Unpaginated.
// It is an intention-revealing alias for [NoPagination].
func Unpaginated() *PaginationSpec {
	return NoPagination()
}

// DefaultPagination returns a PaginationSpec with default page size.
func DefaultPagination() *PaginationSpec {
	return NewPaginationSpecWithSize(PageSizeDefault)
}

// String returns string representation of pagination.
func (p *PaginationSpec) String() string {
	if p.Offset.Some() {
		if p.PageSize.Some() {
			return fmt.Sprintf("offset=%d,size=%d", p.Offset.Unwrap(), p.PageSize.Unwrap())
		}
		return fmt.Sprintf("offset=%d", p.Offset.Unwrap())
	}
	if p.PageNum.Some() && p.PageSize.Some() {
		return fmt.Sprintf("page=%d,size=%d", p.PageNum.Unwrap(), p.PageSize.Unwrap())
	}
	if p.PageNum.Some() {
		return fmt.Sprintf("page=%d", p.PageNum.Unwrap())
	}
	if p.PageSize.Some() {
		return fmt.Sprintf("size=%d", p.PageSize.Unwrap())
	}
	return "no_pagination"
}

// Clone creates a deep copy of the PaginationSpec.
func (p *PaginationSpec) Clone() *PaginationSpec {
	if p == nil {
		return nil
	}
	return &PaginationSpec{
		PageNum:  p.PageNum,
		PageSize: p.PageSize,
		Offset:   p.Offset,
	}
}

// MergeWith merges this pagination with another, with other taking precedence.
func (p *PaginationSpec) MergeWith(other *PaginationSpec) *PaginationSpec {
	result := p.Clone()

	// PageNum and Offset are mutually exclusive position encodings: whichever the
	// other spec sets wins and clears the other, so a stale base position cannot
	// shadow a newer override in ToLimitOffset.
	switch {
	case other.Offset.Some():
		result.Offset = other.Offset
		result.PageNum = maybe.None[int]()
	case other.PageNum.Some():
		result.PageNum = other.PageNum
		result.Offset = maybe.None[int]()
	}
	if other.PageSize.Some() {
		result.PageSize = other.PageSize
	}

	return result
}

// IsPaginated returns true if pagination is configured.
func (p *PaginationSpec) IsPaginated() bool {
	return p.PageNum.Some() || p.PageSize.Some() || p.Offset.Some()
}

// GetPageNum returns the page number (1-indexed), defaulting to 1 if not set.
func (p *PaginationSpec) GetPageNum() int {
	if p.PageNum.Some() {
		pageNum := p.PageNum.Unwrap()
		if pageNum <= 0 {
			return 1 // Pages are 1-indexed, treat 0 or negative as page 1
		}
		return pageNum
	}
	return 1 // Default to first page
}

// GetPageSize returns the page size, defaulting to PageSizeDefault if not set.
func (p *PaginationSpec) GetPageSize() int {
	if p.PageSize.Some() {
		return p.PageSize.Unwrap()
	}
	return PageSizeDefault
}

// GetOffset returns the raw row offset when one was set explicitly (negative
// clamped to 0). The bool reports whether an explicit offset is present; when
// false the caller should derive the offset from PageNum instead.
func (p *PaginationSpec) GetOffset() (int, bool) {
	if !p.Offset.Some() {
		return 0, false
	}
	if offset := p.Offset.Unwrap(); offset > 0 {
		return offset, true
	}
	return 0, true // Offsets are 0-based; treat negative as the first row.
}

// ToLimitOffset converts the spec to Limit/Offset for database queries.
func (p *PaginationSpec) ToLimitOffset() (int, int) {
	limit := p.GetPageSize()

	// A raw offset is carried through verbatim (no page rounding), so a starting
	// row that is not a multiple of the limit is expressible - see GetOffset.
	if offset, ok := p.GetOffset(); ok {
		return limit, offset
	}

	offset := (p.GetPageNum() - 1) * limit // Convert 1-indexed page to 0-indexed offset
	return limit, offset
}

// FinalizeCount returns (entities, totalCount): the backend's paginatedCount when
// pagination was active, otherwise len(entities). Shared by every engine.
func FinalizeCount[T any](entities []T, paginatedCount int64, isPaginated bool) ([]T, int64) {
	if !isPaginated {
		return entities, int64(len(entities))
	}
	return entities, paginatedCount
}
