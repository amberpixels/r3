package r3url

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

// ParseFilters parses a JSON-encoded filter string into r3.Filters.
// The string should be a JSON array of filter objects using the r3json schema.
//
// Example: [{"f":"status","op":"eq","v":"active"},{"f":"age","op":"gte","v":18}]
//
// Empty string returns nil (no filters specified).
func ParseFilters(raw string) (r3.Filters, error) {
	if raw == "" {
		return nil, nil
	}

	var jsonFilters r3json.JSONFilters
	if err := json.Unmarshal([]byte(raw), &jsonFilters); err != nil {
		return nil, newError(fmt.Errorf("failed to parse filters JSON: %w", err))
	}

	filters, err := r3json.JSONToFilters(jsonFilters)
	if err != nil {
		return nil, newError(fmt.Errorf("failed to convert filters: %w", err))
	}
	return filters, nil
}

// FormatFilters formats r3.Filters into a JSON string.
func FormatFilters(filters r3.Filters) (string, error) {
	if len(filters) == 0 {
		return "", nil
	}

	jsonFilters := make(r3json.JSONFilters, 0, len(filters))
	for _, f := range filters {
		jf, err := r3json.FilterToJSON(f)
		if err != nil {
			return "", newError(fmt.Errorf("failed to convert filter: %w", err))
		}
		jsonFilters = append(jsonFilters, jf)
	}

	data, err := json.Marshal(jsonFilters)
	if err != nil {
		return "", newError(fmt.Errorf("failed to marshal filters JSON: %w", err))
	}
	return string(data), nil
}
