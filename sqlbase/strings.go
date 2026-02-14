package sqlbase

import (
	"strings"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/internal/r3lib"
)

// ToSnakeCase converts a name (PascalCase, camelCase, etc.) to snake_case.
// It correctly handles acronyms like "CityID" → "city_id".
func ToSnakeCase(s string) string {
	return r3lib.ToSnakeCase(s)
}

// ToSnakeCasePlural converts a name to snake_case and applies naive pluralization.
func ToSnakeCasePlural(s string) string {
	return r3lib.ToSnakeCasePlural(s)
}

// ColumnsString joins column names with commas.
func ColumnsString(cols []string) string {
	return strings.Join(cols, ", ")
}

// FieldsToColumns converts r3.Fields to a []string of column names.
func FieldsToColumns(fields r3.Fields) []string {
	if len(fields) == 0 {
		return nil
	}
	cols := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != nil {
			cols = append(cols, f.String())
		}
	}
	return cols
}
