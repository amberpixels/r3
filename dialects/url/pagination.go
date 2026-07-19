package r3url

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/amberpixels/r3"
)

// ParsePagination reads the pagination params into a [r3.PaginationSpec], or nil
// when none is present. It accepts either a page number (paramNames.PageNum) or a
// raw 0-based row offset (paramNames.Offset), each paired with an optional size
// (paramNames.PageSize). Page and offset are mutually exclusive; supplying both
// is an error.
func ParsePagination(values url.Values, paramNames ParamNames) (*r3.PaginationSpec, error) {
	rawPageNum := values.Get(paramNames.PageNum)
	rawPageSize := values.Get(paramNames.PageSize)
	rawOffset := values.Get(paramNames.Offset)

	if rawPageNum == "" && rawPageSize == "" && rawOffset == "" {
		return nil, nil //nolint:nilnil // nil pagination means "no pagination requested"
	}

	if rawPageNum != "" && rawOffset != "" {
		return nil, newError(fmt.Errorf(
			"%s and %s are mutually exclusive", paramNames.PageNum, paramNames.Offset))
	}

	pageSize, err := parseNonNegative(rawPageSize, paramNames.PageSize)
	if err != nil {
		return nil, err
	}

	// Offset shape: carry the raw offset verbatim, paired with the size as limit.
	// A missing or zero size means "use the default" (consistent with the page
	// path below), so an offset window is never silently limited to zero rows.
	if rawOffset != "" {
		offset, oErr := parseNonNegative(rawOffset, paramNames.Offset)
		if oErr != nil {
			return nil, oErr
		}
		if pageSize > 0 {
			return r3.NewOffsetPagination(offset, pageSize), nil
		}
		return r3.NewOffsetPagination(offset), nil
	}

	pageNum, err := parseNonNegative(rawPageNum, paramNames.PageNum)
	if err != nil {
		return nil, err
	}

	// Both zero: no pagination requested.
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

// parseNonNegative parses a param that must be a non-negative integer, returning
// 0 for an empty (absent) value.
func parseNonNegative(raw, name string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, newError(fmt.Errorf("invalid %s value %q: %w", name, raw, err))
	}
	if n < 0 {
		return 0, newError(fmt.Errorf("invalid %s value %d: must be non-negative", name, n))
	}
	return n, nil
}

// FormatPagination writes pagination params into url.Values; nil pagination adds none.
func FormatPagination(p *r3.PaginationSpec, paramNames ParamNames) url.Values {
	values := make(url.Values)
	if p == nil {
		return values
	}

	// An offset-based spec round-trips through the offset param; otherwise emit
	// the page number. (The two are mutually exclusive - see PaginationSpec.)
	if offset, ok := p.GetOffset(); ok {
		values.Set(paramNames.Offset, strconv.Itoa(offset))
	} else if pageNum := p.GetPageNum(); pageNum > 0 {
		values.Set(paramNames.PageNum, strconv.Itoa(pageNum))
	}
	if pageSize := p.GetPageSize(); pageSize > 0 {
		values.Set(paramNames.PageSize, strconv.Itoa(pageSize))
	}
	return values
}
