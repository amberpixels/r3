package r3

import (
	"fmt"

	"github.com/amberpixels/k1/maybe"
)

const (
	// PageSizeDefault is a GLOBAL per-package default page size if no pagination was specified.
	PageSizeDefault = 50
)

// PaginationSpec is the core pagination type using PageNum/PageSize.
type PaginationSpec struct {
	// PageNum is 1-indexed page number (1 = first page).
	PageNum maybe.Int
	// PageSize is the number of items per page.
	PageSize maybe.Int
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

// NoPagination returns a PaginationSpec with no pagination limits.
func NoPagination() *PaginationSpec {
	return &PaginationSpec{}
}

// DefaultPagination returns a PaginationSpec with default page size.
func DefaultPagination() *PaginationSpec {
	return NewPaginationSpecWithSize(PageSizeDefault)
}

// String returns string representation of pagination.
func (p *PaginationSpec) String() string {
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
	}
}

// MergeWith merges this pagination with another, with other taking precedence.
func (p *PaginationSpec) MergeWith(other *PaginationSpec) *PaginationSpec {
	result := p.Clone()

	if other.PageNum.Some() {
		result.PageNum = other.PageNum
	}
	if other.PageSize.Some() {
		result.PageSize = other.PageSize
	}

	return result
}

// IsPaginated returns true if pagination is configured.
func (p *PaginationSpec) IsPaginated() bool {
	return p.PageNum.Some() || p.PageSize.Some()
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

// ToLimitOffset converts PageNum/PageSize to Limit/Offset for database queries.
func (p *PaginationSpec) ToLimitOffset() (int, int) {
	pageNum := p.GetPageNum()
	pageSize := p.GetPageSize()

	limit := pageSize
	offset := (pageNum - 1) * pageSize // Convert 1-indexed page to 0-indexed offset

	return limit, offset
}
