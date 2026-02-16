package r3toml

import (
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// TOMLFilter represents a filter criteria in TOML format.
// Uses human-readable field names (field/operator/value) for configuration files.
type TOMLFilter struct {
	Field    string        `toml:"field,omitempty"`
	Operator string        `toml:"operator,omitempty"` // canonical: "eq", "gt", etc.
	Value    any           `toml:"value,omitempty"`
	And      []*TOMLFilter `toml:"and,omitempty"`
	Or       []*TOMLFilter `toml:"or,omitempty"`
}

// TOMLFilters is a slice of TOMLFilter.
type TOMLFilters []*TOMLFilter

// ToFilterSpec converts a TOMLFilter to an r3.FilterSpec.
func (tf *TOMLFilter) ToFilterSpec() (*r3.FilterSpec, error) {
	if tf == nil {
		return nil, newError(fmt.Errorf("nil TOML filter"))
	}

	// Convert AND children
	var andFilters r3.Filters
	if len(tf.And) > 0 {
		andFilters = make(r3.Filters, len(tf.And))
		for i, child := range tf.And {
			f, err := child.ToFilterSpec()
			if err != nil {
				return nil, err
			}
			andFilters[i] = f
		}
	}

	// Convert OR children
	var orFilters r3.Filters
	if len(tf.Or) > 0 {
		orFilters = make(r3.Filters, len(tf.Or))
		for i, child := range tf.Or {
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
	if tf.Field == "" {
		return nil, newError(fmt.Errorf("empty field in TOML filter"))
	}

	op, err := canonical.ParseFilterOperator(tf.Operator)
	if err != nil {
		return nil, newError(fmt.Errorf("failed to parse operator %q: %w", tf.Operator, err))
	}

	return &r3.FilterSpec{
		Field:    r3.NewFieldSpec(tf.Field),
		Operator: op,
		Value:    tf.Value,
	}, nil
}

// toFilters converts a slice of TOMLFilters to r3.Filters.
func (tfs TOMLFilters) toFilters() (r3.Filters, error) {
	filters := make(r3.Filters, len(tfs))
	for i, tf := range tfs {
		f, err := tf.ToFilterSpec()
		if err != nil {
			return nil, err
		}
		filters[i] = f
	}
	return filters, nil
}
