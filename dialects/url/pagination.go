package r3url

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/amberpixels/r3"
)

// ParsePagination extracts pagination parameters from url.Values.
// It reads the page number and page size parameters using the configured param names.
//
// If neither parameter is present, it returns nil (no pagination specified).
func ParsePagination(values url.Values, paramNames ParamNames) (*r3.PaginationSpec, error) {
	rawPageNum := values.Get(paramNames.PageNum)
	rawPageSize := values.Get(paramNames.PageSize)

	if rawPageNum == "" && rawPageSize == "" {
		return nil, nil
	}

	var pageNum, pageSize int
	var err error

	if rawPageNum != "" {
		pageNum, err = strconv.Atoi(rawPageNum)
		if err != nil {
			return nil, newError(fmt.Errorf("invalid %s value %q: %w", paramNames.PageNum, rawPageNum, err))
		}
		if pageNum < 0 {
			return nil, newError(fmt.Errorf("invalid %s value %d: must be non-negative", paramNames.PageNum, pageNum))
		}
	}

	if rawPageSize != "" {
		pageSize, err = strconv.Atoi(rawPageSize)
		if err != nil {
			return nil, newError(fmt.Errorf("invalid %s value %q: %w", paramNames.PageSize, rawPageSize, err))
		}
		if pageSize < 0 {
			return nil, newError(fmt.Errorf("invalid %s value %d: must be non-negative", paramNames.PageSize, pageSize))
		}
	}

	// Both zero means no pagination requested
	if pageNum <= 0 && pageSize <= 0 {
		return r3.NoPagination(), nil
	}

	switch {
	case pageNum > 0 && pageSize > 0:
		return r3.NewPaginationSpec(pageNum, pageSize), nil
	case pageNum > 0:
		return r3.NewPaginationSpec(pageNum), nil
	default:
		return r3.NewPaginationSpecWithSize(pageSize), nil
	}
}

// FormatPagination writes pagination parameters into url.Values.
// If pagination is nil, no parameters are added.
func FormatPagination(p *r3.PaginationSpec, paramNames ParamNames) url.Values {
	values := make(url.Values)
	if p == nil {
		return values
	}

	pageNum := p.GetPageNum()
	pageSize := p.GetPageSize()

	if pageNum > 0 {
		values.Set(paramNames.PageNum, strconv.Itoa(pageNum))
	}
	if pageSize > 0 {
		values.Set(paramNames.PageSize, strconv.Itoa(pageSize))
	}
	return values
}
