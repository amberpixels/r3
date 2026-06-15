package r3url

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// ParseDjangoFilters parses Django-style query parameters into r3.Filters.
//
// Django-style parameters use the pattern "field__operator=value":
//
//	?status=active          -> {f: "status", op: "eq", v: "active"}
//	?age__gte=18            -> {f: "age", op: "gte", v: "18"}
//	?name__like=John%       -> {f: "name", op: "like", v: "John%"}
//	?tags__in=a,b,c         -> {f: "tags", op: "in", v: ["a","b","c"]}
//
// Parameters without the separator are treated as "eq" filters.
// Reserved parameter names (fields, filters, sort, page, page_size, query) are skipped.
//
// The cfg.DjangoFields whitelist controls which fields are allowed.
// If it is empty, all fields are allowed (validated for identifier safety).
func ParseDjangoFilters(values url.Values, cfg Config) (r3.Filters, error) {
	if !cfg.Filter.AllowDjangoStyle {
		return nil, nil
	}

	sep := cfg.Filter.DjangoSeparator
	if sep == "" {
		sep = "__"
	}

	reserved := cfg.reservedParamNames()
	allowed := buildAllowedFieldsMap(cfg.Filter.DjangoFields)

	// Iterate keys in sorted order: ranging a map is randomized, which would make
	// the resulting filter order non-deterministic and break any downstream
	// filter-hashing/caching. The dialect must be pure and deterministic.
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var filters r3.Filters
	for _, key := range keys {
		vals := values[key]
		if len(vals) == 0 {
			continue
		}

		// Skip reserved parameter names
		if _, ok := reserved[key]; ok {
			continue
		}

		field, opStr := splitDjangoParam(key, sep)
		if field == "" {
			continue
		}

		// Check if field is allowed
		if allowed != nil {
			if _, ok := allowed[field]; !ok {
				continue
			}
		}

		// Validate field name for safety
		if err := r3.ValidateIdentifier(field); err != nil {
			return nil, newError(fmt.Errorf("invalid Django-style field %q: %w", field, err))
		}

		// Parse operator
		op := r3.OperatorEq
		if opStr != "" {
			parsed, err := canonical.ParseFilterOperator(opStr)
			if err != nil {
				// Unknown operator — skip this param (it's probably not a filter)
				continue
			}
			op = parsed
		}

		// Parse value
		rawValue := vals[0] // Use first value
		value := parseDjangoValue(rawValue, op)

		filters = append(filters, &r3.FilterSpec{
			Field:    r3.NewFieldSpec(field),
			Operator: op,
			Value:    value,
		})
	}

	return filters, nil
}

// FormatDjangoFilters formats r3.Filters as Django-style URL parameters.
// Only simple filters (non-AND/OR) can be represented in Django-style.
// Filters with AND/OR groups are silently skipped.
func FormatDjangoFilters(filters r3.Filters, cfg Config) url.Values {
	values := make(url.Values)
	if !cfg.Filter.AllowDjangoStyle {
		return values
	}

	sep := cfg.Filter.DjangoSeparator
	if sep == "" {
		sep = "__"
	}

	for _, f := range filters {
		if f == nil || f.Field == nil {
			continue
		}
		// Skip AND/OR group filters — not representable in Django style
		if len(f.And) > 0 || len(f.Or) > 0 {
			continue
		}

		field := f.Field.String()
		opStr := canonical.FormatFilterOperator(f.Operator)

		var key string
		if f.Operator == r3.OperatorEq {
			// eq is the default, use bare field name
			key = field
		} else {
			key = field + sep + opStr
		}

		values.Set(key, formatDjangoValue(f.Value))
	}
	return values
}

// splitDjangoParam splits a parameter key into field name and operator string.
// If there's no separator, the whole key is the field and operator is "".
func splitDjangoParam(key, sep string) (string, string) {
	idx := strings.LastIndex(key, sep)
	if idx < 0 {
		return key, ""
	}

	field := key[:idx]
	op := key[idx+len(sep):]

	// If the operator part is empty or not a recognized operator, treat the whole key as field
	if op == "" {
		return key, ""
	}

	// Check if it's actually a valid operator
	if _, err := canonical.ParseFilterOperator(op); err != nil {
		// Not a valid operator — the separator might be part of the field name
		return key, ""
	}

	return field, op
}

// parseDjangoValue parses a string value based on the operator.
// For "in" and "not_in" operators, it splits by comma to create a slice.
func parseDjangoValue(raw string, op r3.FilterOperatorSpec) any {
	//nolint:exhaustive // only checking list-type operators
	switch op {
	case r3.OperatorIn, r3.OperatorNotIn:
		parts := strings.Split(raw, ",")
		result := make([]any, len(parts))
		for i, p := range parts {
			result[i] = strings.TrimSpace(p)
		}
		return result
	default:
		return raw
	}
}

// formatDjangoValue formats a filter value as a string suitable for URL parameters.
func formatDjangoValue(v any) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = fmt.Sprintf("%v", item)
		}
		return strings.Join(parts, ",")
	case []string:
		return strings.Join(val, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildAllowedFieldsMap converts a slice of allowed fields to a map for O(1) lookups.
// Returns nil if the input is empty (meaning all fields allowed).
func buildAllowedFieldsMap(fields []string) map[string]struct{} {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		m[f] = struct{}{}
	}
	return m
}
