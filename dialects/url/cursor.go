package r3url

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/amberpixels/r3"
)

// ParseCursorPagination reads the after/before/limit params into a [r3.CursorSpec],
// or nil when none are present.
func ParseCursorPagination(values url.Values, paramNames ParamNames) (*r3.CursorSpec, error) {
	after := values.Get(paramNames.CursorAfter)
	before := values.Get(paramNames.CursorBefore)
	rawLimit := values.Get(paramNames.CursorLimit)

	if after == "" && before == "" && rawLimit == "" {
		return nil, nil //nolint:nilnil // nil cursor means "no cursor pagination requested"
	}

	var limit int
	if rawLimit != "" {
		var err error
		limit, err = strconv.Atoi(rawLimit)
		if err != nil {
			return nil, newError(fmt.Errorf("invalid %s value %q: %w", paramNames.CursorLimit, rawLimit, err))
		}
		if limit < 0 {
			return nil, newError(fmt.Errorf("invalid %s value %d: must be non-negative", paramNames.CursorLimit, limit))
		}
	}

	return &r3.CursorSpec{
		After:  after,
		Before: before,
		Limit:  limit,
	}, nil
}

// FormatCursorPagination writes cursor params into url.Values; nil cursor adds none.
func FormatCursorPagination(c *r3.CursorSpec, paramNames ParamNames) url.Values {
	values := make(url.Values)
	if c == nil {
		return values
	}

	if c.After != "" {
		values.Set(paramNames.CursorAfter, c.After)
	}
	if c.Before != "" {
		values.Set(paramNames.CursorBefore, c.Before)
	}
	if c.Limit > 0 {
		values.Set(paramNames.CursorLimit, strconv.Itoa(c.Limit))
	}
	return values
}
