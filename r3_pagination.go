package r3

import (
	"fmt"

	"github.com/amberpixels/k1/maybe"
)

const (
	// PageSizeDefault is a GLOBAL per-package default page size if no pagination was specified.
	PageSizeDefault = 50
)

// Pagination represents pagination parameters using Visitor pattern.
type Pagination interface {
	String() string
	Clone() Pagination
	// ToDialect converts pagination to dialect-specific representation.
	ToDialect(dialector PaginationDialector) (DialectValue, error)
}

// PaginationDialector defines the interface for converting pagination to/from dialect formats.
type PaginationDialector interface {
	// TranslateIntoPaginationSpec converts a dialect value to PaginationSpec.
	TranslateIntoPaginationSpec(DialectValue) (*PaginationSpec, error)
	// TranslatePaginationSpec converts PaginationSpec to dialect value.
	TranslatePaginationSpec(*PaginationSpec) (DialectValue, error)
}

// PaginationSpec is the core implementation of Pagination using PageNum/PageSize.
type PaginationSpec struct {
	// PageNum is 1-indexed page number (1 = first page).
	PageNum maybe.Int
	// PageSize is the number of items per page.
	PageSize maybe.Int
}

// Ensure PaginationSpec implements Pagination interface.
var _ Pagination = (*PaginationSpec)(nil)

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
func (p *PaginationSpec) Clone() Pagination {
	return &PaginationSpec{
		PageNum:  p.PageNum,
		PageSize: p.PageSize,
	}
}

// ToDialect converts PaginationSpec to dialect-specific representation.
func (p *PaginationSpec) ToDialect(dialector PaginationDialector) (DialectValue, error) {
	return dialector.TranslatePaginationSpec(p)
}

// MergeWith merges this pagination with another, with other taking precedence.
func (p *PaginationSpec) MergeWith(other *PaginationSpec) *PaginationSpec {
	result, _ := p.Clone().(*PaginationSpec)

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

// FromDialect creates a PaginationSpec from a dialect value.
func FromDialect(dialector PaginationDialector, dialectValue DialectValue) (*PaginationSpec, error) {
	return dialector.TranslateIntoPaginationSpec(dialectValue)
}
