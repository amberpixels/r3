// Package r3utils provides shared utility functions used across r3 packages.
package r3utils

import (
	"strings"

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
