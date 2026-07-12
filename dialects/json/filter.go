package r3json

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/r3"
)

// JSONFilter represents a filter in JSON format with field, operator, value, and logical groups.
type JSONFilter struct {
	Field JSONField `json:"f,omitempty"`

	Op    JSONFilterOperator `json:"op,omitempty"`
	Value any                `json:"v,omitempty"` // raw value

	// Logical groups:
	And JSONFilters `json:"and,omitempty"`
	Or  JSONFilters `json:"or,omitempty"`
}

// JSONFilters is a slice of JSONFilter.
type JSONFilters []*JSONFilter

// String returns a string representation of a single filter (as a json string).
func (jf *JSONFilter) String() string {
	jj, err := json.Marshal(jf)
	if err != nil {
		return fmt.Sprintf("<corrupted filters (%s)>", err.Error())
	}

	return string(jj)
}

// String returns string representation of the JSON Filters (as a json string).
func (jfs JSONFilters) String() string {
	jj, err := json.Marshal(jfs)
	if err != nil {
		return fmt.Sprintf("<corrupted filters (%s)>", err.Error())
	}

	return string(jj)
}

// MarshalJSON implements custom JSON marshaling to handle nil values correctly.
func (jf *JSONFilter) MarshalJSON() ([]byte, error) {
	type alias JSONFilter

	// AND/OR groups marshal by default; simple filters go through a map below
	// so a nil "v" is emitted explicitly rather than omitted.
	if len(jf.And) > 0 || len(jf.Or) > 0 {
		return json.Marshal((*alias)(jf))
	}

	result := make(map[string]any)

	if jf.Field != "" {
		result["f"] = jf.Field
	}
	if jf.Op != OperatorUnspecified {
		result["op"] = jf.Op
	}

	// Always emit "v" for a simple filter, even when nil.
	if jf.Field != "" && jf.Op != OperatorUnspecified {
		result["v"] = jf.Value
	}

	return json.Marshal(result)
}

// UnmarshalJSON implements json.Unmarshaler.
func (jf *JSONFilter) UnmarshalJSON(data []byte) error {
	type alias JSONFilter
	return json.Unmarshal(data, (*alias)(jf))
}

func (jf *JSONFilter) ToFilterSpec() (*r3.FilterSpec, error) {
	andFilters, err := jf.And.toFilters()
	if err != nil {
		return nil, err
	}
	orFilters, err := jf.Or.toFilters()
	if err != nil {
		return nil, err
	}

	// AND/OR groups skip field-level validation.
	if len(andFilters) > 0 || len(orFilters) > 0 {
		return &r3.FilterSpec{
			And: andFilters,
			Or:  orFilters,
		}, nil
	}

	fieldSpec, err := jf.Field.ToFieldSpec()
	if err != nil {
		return nil, err
	}

	opSpec, err := jf.Op.ToFilterOperatorSpec()
	if err != nil {
		return nil, err
	}

	return &r3.FilterSpec{
		Field:    fieldSpec,
		Operator: opSpec,
		Value:    jf.Value,

		And: andFilters,
		Or:  orFilters,
	}, nil
}

func (jfs JSONFilters) toFilters() (r3.Filters, error) {
	r3Filters := make(r3.Filters, len(jfs))
	for i, f := range jfs {
		filterSpecs, err := f.ToFilterSpec()
		if err != nil {
			return nil, err
		}
		r3Filters[i] = filterSpecs
	}
	return r3Filters, nil
}
