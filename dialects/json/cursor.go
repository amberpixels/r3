package r3json

import "github.com/amberpixels/r3"

// JSONCursorPagination represents cursor-based pagination in JSON format.
type JSONCursorPagination struct {
	After  string `json:"after,omitempty"`
	Before string `json:"before,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// ToCursorSpec converts JSONCursorPagination to an r3.CursorSpec.
// Returns nil if no cursor parameters are set.
func (jc *JSONCursorPagination) ToCursorSpec() *r3.CursorSpec {
	if jc.After == "" && jc.Before == "" && jc.Limit <= 0 {
		return nil
	}
	return &r3.CursorSpec{
		After:  jc.After,
		Before: jc.Before,
		Limit:  jc.Limit,
	}
}

// CursorToJSON converts an r3.CursorSpec to JSONCursorPagination.
// Returns nil if the cursor is nil.
func CursorToJSON(c *r3.CursorSpec) *JSONCursorPagination {
	if c == nil {
		return nil
	}
	return &JSONCursorPagination{
		After:  c.After,
		Before: c.Before,
		Limit:  c.Limit,
	}
}
