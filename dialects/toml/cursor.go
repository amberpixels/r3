package r3toml

import "github.com/amberpixels/r3"

// TOMLCursorPagination represents cursor-based pagination in TOML format.
type TOMLCursorPagination struct {
	After  string `toml:"after,omitempty"`
	Before string `toml:"before,omitempty"`
	Limit  int    `toml:"limit,omitempty"`
}

// ToCursorSpec converts TOMLCursorPagination to an r3.CursorSpec.
// Returns nil if no cursor parameters are set.
func (tc *TOMLCursorPagination) ToCursorSpec() *r3.CursorSpec {
	if tc.After == "" && tc.Before == "" && tc.Limit <= 0 {
		return nil
	}
	return &r3.CursorSpec{
		After:  tc.After,
		Before: tc.Before,
		Limit:  tc.Limit,
	}
}
