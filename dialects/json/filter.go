package r3json

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/r3"
)

type JSONFilter struct {
	Field JSONField `json:"f,omitempty"`

	// The operator is provided as a string (like "eq", "ne", etc.)
	Op    JSONFilterOperator `json:"op,omitempty"`
	Value any                `json:"v,omitempty"` // allow for raw value handling (or use any, string, etc.)

	// Logical groups:
	And JSONFilters `json:"and,omitempty"`
	Or  JSONFilters `json:"or,omitempty"`
}

// JSONFilters is a slice of JSONFilter.
type JSONFilters []*JSONFilter

// TODO: validate filters

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

// UnmarshalJSON is optional, depending on how you want to handle the Value.
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
