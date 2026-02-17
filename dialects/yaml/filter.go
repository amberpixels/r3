package r3yaml

import (
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// YAMLFilter represents a filter criteria in YAML format.
// Uses human-readable field names (field/operator/value) for configuration files.
type YAMLFilter struct {
	Field    string        `yaml:"field,omitempty"`
	Operator string        `yaml:"operator,omitempty"` // canonical: "eq", "gt", etc.
	Value    any           `yaml:"value,omitempty"`
	And      []*YAMLFilter `yaml:"and,omitempty"`
	Or       []*YAMLFilter `yaml:"or,omitempty"`
}

// YAMLFilters is a slice of YAMLFilter.
type YAMLFilters []*YAMLFilter

// ToFilterSpec converts a YAMLFilter to an r3.FilterSpec.
func (yf *YAMLFilter) ToFilterSpec() (*r3.FilterSpec, error) {
	if yf == nil {
		return nil, newError(errors.New("nil YAML filter"))
	}

	// Convert AND children
	var andFilters r3.Filters
	if len(yf.And) > 0 {
		andFilters = make(r3.Filters, len(yf.And))
		for i, child := range yf.And {
			f, err := child.ToFilterSpec()
			if err != nil {
				return nil, err
			}
			andFilters[i] = f
		}
	}

	// Convert OR children
	var orFilters r3.Filters
	if len(yf.Or) > 0 {
		orFilters = make(r3.Filters, len(yf.Or))
		for i, child := range yf.Or {
			f, err := child.ToFilterSpec()
			if err != nil {
				return nil, err
			}
			orFilters[i] = f
		}
	}

	// For AND/OR group filters, don't need field-level validation
	if len(andFilters) > 0 || len(orFilters) > 0 {
		return &r3.FilterSpec{
			And: andFilters,
			Or:  orFilters,
		}, nil
	}

	// For simple filters, parse field and operator
	if yf.Field == "" {
		return nil, newError(errors.New("empty field in YAML filter"))
	}

	op, err := canonical.ParseFilterOperator(yf.Operator)
	if err != nil {
		return nil, newError(fmt.Errorf("failed to parse operator %q: %w", yf.Operator, err))
	}

	return &r3.FilterSpec{
		Field:    r3.NewFieldSpec(yf.Field),
		Operator: op,
		Value:    yf.Value,
	}, nil
}

// toFilters converts a slice of YAMLFilters to r3.Filters.
func (yfs YAMLFilters) toFilters() (r3.Filters, error) {
	filters := make(r3.Filters, len(yfs))
	for i, yf := range yfs {
		f, err := yf.ToFilterSpec()
		if err != nil {
			return nil, err
		}
		filters[i] = f
	}
	return filters, nil
}
