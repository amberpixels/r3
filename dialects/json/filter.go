package r3json

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/r3"
)

type Filter struct {
	Field Field `json:"f,omitempty"`
	// The operator is provided as a string (like "eq", "ne", etc.)
	Op    FilterOperator `json:"op,omitempty"`
	Value any            `json:"v,omitempty"` // allow for raw value handling (or use any, string, etc.)

	// Logical groups:
	And Filters `json:"and,omitempty"`
	Or  Filters `json:"or,omitempty"`
}

// TODO: validate filters

type Filters []*Filter

func (jsf Filters) String() string {
	jj, err := json.Marshal(jsf)
	if err != nil {
		return fmt.Sprintf("<corrupted filters (%s)>", err.Error())
	}

	return string(jj)
}

func (d *JsonInboundDialector) ToFilter(dialectValue r3.DialectValue) (r3.Filter, error) {
	inboundFilter, ok := dialectValue.(*Filter)
	if !ok {
		return nil, fmt.Errorf("invalid filter type: %T", dialectValue)
	}

	return inboundFilter.ToColumnFilter()
}

func (d *JsonInboundDialector) ToFilters(dialectValue r3.DialectValue) (r3.Filters, error) {
	inboundFilters, ok := dialectValue.(Filters)
	if !ok {
		inboundFilter, ok := dialectValue.(*Filter)
		if !ok {
			return nil, fmt.Errorf("invalid filter type: %T", dialectValue)
		}

		inboundFilters = Filters{inboundFilter}
	}

	return inboundFilters.ToColumnFilters()
}

func JsonFiltersToFilters(filters Filters) (r3.Filters, error) {
	return (&JsonInboundDialector{}).ToFilters(filters)
}

// UnmarshalJSON is optional, depending on how you want to handle the Value.
func (jf *Filter) UnmarshalJSON(data []byte) error {
	type alias Filter
	return json.Unmarshal(data, (*alias)(jf))
}

func (jsf Filters) ToColumnFilters() (r3.Filters, error) {
	columnFilters := make(r3.Filters, len(jsf))
	for i, f := range jsf {
		columnFilter, err := f.ToColumnFilter()
		if err != nil {
			return nil, err
		}
		columnFilters[i] = columnFilter
	}
	return columnFilters, nil
}

func (jf *Filter) ToColumnFilter() (*r3.ColumnFilter, error) {
	andFilters, err := jf.And.ToColumnFilters()
	if err != nil {
		return nil, err
	}
	orFilters, err := jf.Or.ToColumnFilters()
	if err != nil {
		return nil, err
	}

	field, err := jf.Field.ToColumnField()
	if err != nil {
		return nil, err
	}

	return &r3.ColumnFilter{
		Field:    field,
		Operator: r3.FilterOperator(jf.Op),
		Value:    jf.Value,

		And: andFilters,
		Or:  orFilters,
	}, nil
}
