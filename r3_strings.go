package r3

import (
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
