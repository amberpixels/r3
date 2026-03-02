package r3url

import (
	"strings"

	"github.com/amberpixels/r3"
)

// ParseFields parses a comma-separated string of field names into r3.Fields.
// Example: "id,name,status" -> Fields{FieldSpec("id"), FieldSpec("name"), FieldSpec("status")}
//
// Empty string returns nil (no fields specified).
func ParseFields(raw string) (r3.Fields, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	fields := make(r3.Fields, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields = append(fields, r3.NewFieldSpec(p))
	}

	if len(fields) == 0 {
		return nil, nil
	}
	return fields, nil
}

// FormatFields formats r3.Fields into a comma-separated string.
// Example: Fields{FieldSpec("id"), FieldSpec("name")} -> "id,name".
func FormatFields(fields r3.Fields) string {
	if len(fields) == 0 {
		return ""
	}

	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.String()
	}
	return strings.Join(parts, ",")
}
