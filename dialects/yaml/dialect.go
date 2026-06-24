package r3yaml

import (
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// FieldToYAML converts a FieldSpec to a YAMLField.
func FieldToYAML(f *r3.FieldSpec) YAMLField {
	if f == nil {
		return ""
	}
	return YAMLField(f.String())
}

// YAMLToField converts a YAMLField to a FieldSpec.
func YAMLToField(yf YAMLField) *r3.FieldSpec {
	return r3.NewFieldSpec(yf.String())
}

// FilterToYAML converts a FilterSpec to a YAMLFilter.
func FilterToYAML(f *r3.FilterSpec) (*YAMLFilter, error) {
	if f == nil {
		return nil, newError(errors.New("nil filter spec"))
	}

	// Relationship ("has") filters are resolved by the driver against the
	// database; they have no serialized form. Fail loudly rather than silently
	// drop the relation and emit a match-all filter.
	if f.Relation != "" {
		return nil, newError(fmt.Errorf("relationship filter on %q cannot be serialized to YAML", f.Relation))
	}

	yf := &YAMLFilter{}

	// Handle simple field-operator-value filters
	if f.Field != nil {
		yf.Field = f.Field.String()
		yf.Value = f.Value

		name := canonical.FormatFilterOperator(f.Operator)
		if name == "" {
			return nil, newError(fmt.Errorf("unsupported operator: %v", f.Operator))
		}
		yf.Operator = name
	}

	// Handle AND/OR groups recursively
	if len(f.And) > 0 {
		yf.And = make([]*YAMLFilter, len(f.And))
		for i, child := range f.And {
			cf, err := FilterToYAML(child)
			if err != nil {
				return nil, err
			}
			yf.And[i] = cf
		}
	}

	if len(f.Or) > 0 {
		yf.Or = make([]*YAMLFilter, len(f.Or))
		for i, child := range f.Or {
			cf, err := FilterToYAML(child)
			if err != nil {
				return nil, err
			}
			yf.Or[i] = cf
		}
	}

	return yf, nil
}

// YAMLToFilter converts a YAMLFilter to a FilterSpec.
func YAMLToFilter(yf *YAMLFilter) (*r3.FilterSpec, error) {
	return yf.ToFilterSpec()
}

// YAMLToFilters converts a slice of YAMLFilters to r3.Filters.
func YAMLToFilters(yfs YAMLFilters) (r3.Filters, error) {
	return yfs.toFilters()
}

// SortToYAML converts a SortSpec to a YAMLSort.
func SortToYAML(s *r3.SortSpec) *YAMLSort {
	if s == nil {
		return nil
	}
	return &YAMLSort{
		Field:         s.Column.String(),
		Direction:     canonical.FormatSortDirection(s.Direction),
		NullsPosition: canonical.FormatNullsPosition(s.NullsPosition),
	}
}

// YAMLToSort converts a YAMLSort to a SortSpec.
func YAMLToSort(ys *YAMLSort) (*r3.SortSpec, error) {
	return ys.ToSortSpec()
}

// PaginationToYAML converts a PaginationSpec to a YAMLPagination.
func PaginationToYAML(p *r3.PaginationSpec) *YAMLPagination {
	if p == nil {
		return &YAMLPagination{}
	}
	return &YAMLPagination{
		PageNum:  p.GetPageNum(),
		PageSize: p.GetPageSize(),
	}
}

// YAMLToPagination converts a YAMLPagination to a PaginationSpec.
func YAMLToPagination(yp *YAMLPagination) (*r3.PaginationSpec, error) {
	return yp.ToPaginationSpec()
}
