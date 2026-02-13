package sqlbase

import (
	"strings"

	"github.com/amberpixels/r3"
	"github.com/d3rty/json/cases"
)

// ToSnakeCase converts a name (PascalCase, camelCase, etc.) to snake_case.
// It correctly handles acronyms like "CityID" → "city_id".
func ToSnakeCase(s string) string {
	return cases.TransformTo(s, cases.Snake)
}

// ToSnakeCasePlural converts a name to snake_case and applies naive pluralization.
func ToSnakeCasePlural(s string) string {
	snake := ToSnakeCase(s)

	if strings.HasSuffix(snake, "y") {
		return snake[:len(snake)-1] + "ies"
	}
	if strings.HasSuffix(snake, "s") || strings.HasSuffix(snake, "x") ||
		strings.HasSuffix(snake, "sh") || strings.HasSuffix(snake, "ch") {
		return snake + "es"
	}
	return snake + "s"
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
