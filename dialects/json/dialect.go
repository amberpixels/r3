package r3json

import (
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// FieldToJSON converts a FieldSpec to a JSONField.
func FieldToJSON(f *r3.FieldSpec) JSONField {
	if f == nil {
		return ""
	}
	return JSONField(f.String())
}

// JSONToField converts a JSONField to a FieldSpec.
func JSONToField(jf JSONField) *r3.FieldSpec {
	return r3.NewFieldSpec(jf.String())
}

// OperatorToJSON converts a FilterOperatorSpec to a JSONFilterOperator.
func OperatorToJSON(op r3.FilterOperatorSpec) (JSONFilterOperator, error) {
	return r3ToJSONOperator(op)
}

// JSONToOperator converts a JSONFilterOperator to a FilterOperatorSpec.
func JSONToOperator(jop JSONFilterOperator) (r3.FilterOperatorSpec, error) {
	return jop.ToFilterOperatorSpec()
}

// FilterToJSON converts a FilterSpec to a JSONFilter.
func FilterToJSON(f *r3.FilterSpec) (*JSONFilter, error) {
	if f == nil {
		return nil, newError(errors.New("nil filter spec"))
	}

	jsonFilter := &JSONFilter{}

	// Handle simple field-operator-value filters
	if f.Field != nil {
		jsonFilter.Field = JSONField(f.Field.String())
		jsonFilter.Value = f.Value

		jsonOp, err := r3ToJSONOperator(f.Operator)
		if err != nil {
			return nil, newError(fmt.Errorf("failed to convert operator: %w", err))
		}
		jsonFilter.Op = jsonOp
	}

	// Handle AND/OR groups recursively
	if len(f.And) > 0 {
		jsonFilter.And = make(JSONFilters, len(f.And))
		for i, child := range f.And {
			jf, err := FilterToJSON(child)
			if err != nil {
				return nil, err
			}
			jsonFilter.And[i] = jf
		}
	}

	if len(f.Or) > 0 {
		jsonFilter.Or = make(JSONFilters, len(f.Or))
		for i, child := range f.Or {
			jf, err := FilterToJSON(child)
			if err != nil {
				return nil, err
			}
			jsonFilter.Or[i] = jf
		}
	}

	return jsonFilter, nil
}

// JSONToFilter converts a JSONFilter to a FilterSpec.
func JSONToFilter(jf *JSONFilter) (*r3.FilterSpec, error) {
	return jf.ToFilterSpec()
}

// JSONToFilters converts a slice of JSONFilters to r3.Filters.
func JSONToFilters(jfs JSONFilters) (r3.Filters, error) {
	return jfs.toFilters()
}

// JSONFiltersToFilters is an alias for JSONToFilters for backward compatibility.
//
// Deprecated: Use JSONToFilters instead.
func JSONFiltersToFilters(inboundFilters JSONFilters) (r3.Filters, error) {
	return JSONToFilters(inboundFilters)
}

// SortToJSON converts a SortSpec to a JSONSort.
func SortToJSON(s *r3.SortSpec) *JSONSort {
	if s == nil {
		return nil
	}
	return &JSONSort{
		Field:         JSONField(s.Column.String()),
		Direction:     jsonSortDirectionFromR3(s.Direction),
		NullsPosition: jsonNullsPositionFromR3(s.NullsPosition),
	}
}

// JSONToSort converts a JSONSort to a SortSpec.
func JSONToSort(js *JSONSort) (*r3.SortSpec, error) {
	return js.ToSortSpec()
}

// PaginationToJSON converts a PaginationSpec to a JSONPagination.
func PaginationToJSON(p *r3.PaginationSpec) *JSONPagination {
	return jsonPaginationFromR3(p)
}

// JSONToPagination converts a JSONPagination to a PaginationSpec.
func JSONToPagination(jp *JSONPagination) (*r3.PaginationSpec, error) {
	return jp.ToPaginationSpec()
}

// --- Internal helpers ---

// r3ToJSONOperator converts r3 operators to JSON operators.
func r3ToJSONOperator(op r3.FilterOperatorSpec) (JSONFilterOperator, error) {
	name := canonical.FormatFilterOperator(op)
	if name == "" {
		return OperatorUnspecified, fmt.Errorf("unsupported r3 filter operator: %v", op)
	}

	jsonOp, ok := FilterOperatorValues[name]
	if !ok {
		return OperatorUnspecified, fmt.Errorf("unsupported r3 filter operator: %v", op)
	}
	return jsonOp, nil
}
