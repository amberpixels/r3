package r3json

import (
	"encoding/json"
	"strings"

	"github.com/amberpixels/r3"
)

// JSONField is just a string value that represents a field inside a JSON object
// e.g. `{"fields":["id", "status"]}.
type JSONField string

// JSONFields as of collection of json.JSONField's.
type JSONFields []JSONField

// String returns string representation of the field.
func (f *JSONField) String() string { return string(*f) }

// String returns string representation of the fields
// E.g. JSONFields{"id", "status"} => "id,status".
func (fields JSONFields) String() string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.String()
	}
	return strings.Join(parts, ",")
}

// UnmarshalJSON is optional, depending on how you want to handle the Value.
func (f *JSONField) UnmarshalJSON(data []byte) error {
	type alias JSONField
	return json.Unmarshal(data, (*alias)(f))
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// This is used to decode operator from plain text.
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
