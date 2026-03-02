package r3yaml

import "github.com/amberpixels/r3"

// YAMLCursorPagination represents cursor-based pagination in YAML format.
type YAMLCursorPagination struct {
	After  string `yaml:"after,omitempty"`
	Before string `yaml:"before,omitempty"`
	Limit  int    `yaml:"limit,omitempty"`
}

// ToCursorSpec converts YAMLCursorPagination to an r3.CursorSpec.
// Returns nil if no cursor parameters are set.
func (yc *YAMLCursorPagination) ToCursorSpec() *r3.CursorSpec {
	if yc.After == "" && yc.Before == "" && yc.Limit <= 0 {
		return nil
	}
	return &r3.CursorSpec{
		After:  yc.After,
		Before: yc.Before,
		Limit:  yc.Limit,
	}
}
