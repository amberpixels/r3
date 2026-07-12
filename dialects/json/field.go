package r3json

import (
	"encoding/json"
	"strings"

	"github.com/amberpixels/r3"
)

// JSONField is a field name inside a JSON object, e.g. {"fields":["id","status"]}.
type JSONField string

// JSONFields is a collection of JSONField.
type JSONFields []JSONField

// String returns the field name.
func (f *JSONField) String() string { return string(*f) }

// String joins the fields comma-separated, e.g. {"id","status"} => "id,status".
func (fields JSONFields) String() string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.String()
	}
	return strings.Join(parts, ",")
}

// UnmarshalJSON implements json.Unmarshaler.
func (f *JSONField) UnmarshalJSON(data []byte) error {
	type alias JSONField
	return json.Unmarshal(data, (*alias)(f))
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (f *JSONField) UnmarshalText(text []byte) error {
	*f = JSONField(string(text))
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
func (f *JSONField) MarshalText() ([]byte, error) {
	return []byte(f.String()), nil
}

// ToFieldSpec converts a JSON field into r3 FieldSpec.
func (f *JSONField) ToFieldSpec() (*r3.FieldSpec, error) {
	return r3.NewFieldSpec(f.String()), nil
}

// ToFieldSpecs converts list of JSON Fields into r3 Fields.
func (fields JSONFields) ToFieldSpecs() (r3.Fields, error) {
	fieldSpecs := make(r3.Fields, len(fields))
	for i, f := range fields {
		fieldSpec, err := f.ToFieldSpec()
		if err != nil {
			return nil, err
		}
		fieldSpecs[i] = fieldSpec
	}
	return fieldSpecs, nil
}
