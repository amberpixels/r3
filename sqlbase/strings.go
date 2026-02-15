package sqlbase

import (
	"strings"

	"github.com/amberpixels/r3"
)

// ColumnsString joins column names with commas (SQL-specific formatting).
func ColumnsString(cols []string) string {
	return strings.Join(cols, ", ")
}

// FieldsToColumns converts r3.Fields to a []string of column names.
//
// Deprecated: Use r3.FieldsToStrings directly.
func FieldsToColumns(fields r3.Fields) []string {
	return r3.FieldsToStrings(fields)
}
