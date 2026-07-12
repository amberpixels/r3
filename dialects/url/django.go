package r3url

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// ParseDjangoFilters parses "field__operator=value" params into r3.Filters. A param
// with no separator is an "eq" filter; reserved param names are skipped. The
// cfg.DjangoFields whitelist gates fields (empty = all, validated for identifier safety).
//
//	?status=active          -> {f: "status", op: "eq", v: "active"}
//	?age__gte=18            -> {f: "age", op: "gte", v: "18"}
//	?tags__in=a,b,c         -> {f: "tags", op: "in", v: ["a","b","c"]}
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

	// Sort keys: map ranging is randomized, and a non-deterministic filter order
	// would break downstream filter-hashing/caching.
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

		if _, ok := reserved[key]; ok {
			continue
		}

		field, opStr := splitDjangoParam(key, sep)
		if field == "" {
			continue
		}

		if allowed != nil {
			if _, ok := allowed[field]; !ok {
				continue
			}
		}

		if err := r3.ValidateIdentifier(field); err != nil {
			return nil, newError(fmt.Errorf("invalid Django-style field %q: %w", field, err))
		}

		op := r3.OperatorEq
		if opStr != "" {
			parsed, err := canonical.ParseFilterOperator(opStr)
			if err != nil {
				// Unknown operator: probably not a filter, skip.
				continue
			}
			op = parsed
		}

		rawValue := vals[0]
		value := parseDjangoValue(rawValue, op)

		filters = append(filters, &r3.FilterSpec{
			Field:    r3.NewFieldSpec(field),
			Operator: op,
			Value:    value,
		})
	}

	return filters, nil
}

// FormatDjangoFilters formats r3.Filters as Django-style URL parameters. AND/OR group
// filters aren't representable in this style and are silently skipped.
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
		if len(f.And) > 0 || len(f.Or) > 0 {
			continue
		}

		field := f.Field.String()
		opStr := canonical.FormatFilterOperator(f.Operator)

		var key string
		if f.Operator == r3.OperatorEq {
			// eq is the default: bare field name.
			key = field
		} else {
			key = field + sep + opStr
		}

		values.Set(key, formatDjangoValue(f.Value))
	}
	return values
}

// splitDjangoParam splits a key into field and operator. With no separator, or when the
// suffix isn't a valid operator (the separator may be part of the field name), the whole
// key is the field and the operator is "".
func splitDjangoParam(key, sep string) (string, string) {
	idx := strings.LastIndex(key, sep)
	if idx < 0 {
		return key, ""
	}

	field := key[:idx]
	op := key[idx+len(sep):]

	if op == "" {
		return key, ""
	}

	if _, err := canonical.ParseFilterOperator(op); err != nil {
		return key, ""
	}

	return field, op
}

// parseDjangoValue parses a value by operator: in/not_in split on commas into a slice.
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

// buildAllowedFieldsMap indexes allowed fields for O(1) lookup; nil when empty (all allowed).
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
