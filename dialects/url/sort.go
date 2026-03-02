package r3url

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
	r3json "github.com/amberpixels/r3/dialects/json"
)

// ParseSort parses a raw sort string into r3.Sorts using the specified format.
//
// Empty string returns nil (no sorts specified).
func ParseSort(raw string, format SortFormat) (r3.Sorts, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	switch format {
	case SortFormatColonDir:
		return parseSortColonDir(raw)
	case SortFormatSignPrefix:
		return parseSortSignPrefix(raw)
	case SortFormatJSON:
		return parseSortJSON(raw)
	default:
		return nil, newError(fmt.Errorf("unsupported sort format: %d", format))
	}
}

// FormatSort formats r3.Sorts into a string using the specified format.
func FormatSort(sorts r3.Sorts, format SortFormat) (string, error) {
	if len(sorts) == 0 {
		return "", nil
	}

	switch format {
	case SortFormatColonDir:
		return formatSortColonDir(sorts), nil
	case SortFormatSignPrefix:
		return formatSortSignPrefix(sorts), nil
	case SortFormatJSON:
		return formatSortJSON(sorts)
	default:
		return "", newError(fmt.Errorf("unsupported sort format: %d", format))
	}
}

// parseSortColonDir parses "name:asc,age:desc" or "name:asc:nulls_last" format.
func parseSortColonDir(raw string) (r3.Sorts, error) {
	parts := strings.Split(raw, ",")
	sorts := make(r3.Sorts, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		segments := strings.Split(part, ":")
		spec := &r3.SortSpec{}

		// First segment: field name (required)
		field := strings.TrimSpace(segments[0])
		if field == "" {
			return nil, newError(fmt.Errorf("empty field name in sort spec %q", part))
		}
		spec.Column = r3.NewFieldSpec(field)

		// Second segment: direction (optional, defaults to asc)
		if len(segments) >= 2 {
			dirStr := strings.TrimSpace(segments[1])
			spec.Direction = canonical.ParseSortDirection(dirStr)
		} else {
			spec.Direction = r3.SortDirectionAsc
		}

		// Third segment: nulls position (optional)
		if len(segments) >= 3 { //nolint:mnd // field:direction:nulls_position
			nullsStr := strings.TrimSpace(segments[2])
			spec.NullsPosition = canonical.ParseNullsPosition(nullsStr)
		}

		sorts = append(sorts, spec)
	}

	if len(sorts) == 0 {
		return nil, nil
	}
	return sorts, nil
}

// parseSortSignPrefix parses "name,-age" format (- prefix = desc).
func parseSortSignPrefix(raw string) (r3.Sorts, error) {
	parts := strings.Split(raw, ",")
	sorts := make(r3.Sorts, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		spec := &r3.SortSpec{}
		if strings.HasPrefix(part, "-") {
			spec.Direction = r3.SortDirectionDesc
			part = part[1:]
		} else {
			spec.Direction = r3.SortDirectionAsc
			// Also support explicit "+" prefix
			part = strings.TrimPrefix(part, "+")
		}

		part = strings.TrimSpace(part)
		if part == "" {
			return nil, newError(errors.New("empty field name in sort spec"))
		}
		spec.Column = r3.NewFieldSpec(part)

		sorts = append(sorts, spec)
	}

	if len(sorts) == 0 {
		return nil, nil
	}
	return sorts, nil
}

// parseSortJSON parses a JSON array of sort objects.
func parseSortJSON(raw string) (r3.Sorts, error) {
	var jsonSorts []*r3json.JSONSort
	if err := json.Unmarshal([]byte(raw), &jsonSorts); err != nil {
		return nil, newError(fmt.Errorf("failed to parse sort JSON: %w", err))
	}

	sorts := make(r3.Sorts, 0, len(jsonSorts))
	for _, js := range jsonSorts {
		spec, err := r3json.JSONToSort(js)
		if err != nil {
			return nil, newError(fmt.Errorf("failed to convert sort: %w", err))
		}
		sorts = append(sorts, spec)
	}

	if len(sorts) == 0 {
		return nil, nil
	}
	return sorts, nil
}

// formatSortColonDir formats as "name:asc,age:desc" or "name:asc:nulls_last".
func formatSortColonDir(sorts r3.Sorts) string {
	parts := make([]string, 0, len(sorts))
	for _, s := range sorts {
		if s == nil || s.Column == nil {
			continue
		}
		part := s.Column.String()

		dirStr := canonical.FormatSortDirection(s.Direction)
		if dirStr == "" {
			dirStr = "asc"
		}
		part += ":" + dirStr

		nullsStr := canonical.FormatNullsPosition(s.NullsPosition)
		if nullsStr != "" {
			part += ":" + nullsStr
		}

		parts = append(parts, part)
	}
	return strings.Join(parts, ",")
}

// formatSortSignPrefix formats as "name,-age".
func formatSortSignPrefix(sorts r3.Sorts) string {
	parts := make([]string, 0, len(sorts))
	for _, s := range sorts {
		if s == nil || s.Column == nil {
			continue
		}
		field := s.Column.String()
		if s.Direction == r3.SortDirectionDesc {
			field = "-" + field
		}
		parts = append(parts, field)
	}
	return strings.Join(parts, ",")
}

// formatSortJSON formats as a JSON array of sort objects.
func formatSortJSON(sorts r3.Sorts) (string, error) {
	jsonSorts := make([]*r3json.JSONSort, 0, len(sorts))
	for _, s := range sorts {
		if s == nil {
			continue
		}
		jsonSorts = append(jsonSorts, r3json.SortToJSON(s))
	}

	data, err := json.Marshal(jsonSorts)
	if err != nil {
		return "", newError(fmt.Errorf("failed to marshal sort JSON: %w", err))
	}
	return string(data), nil
}
