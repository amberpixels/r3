package r3toml

import (
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// FieldToTOML converts a FieldSpec to a TOMLField.
func FieldToTOML(f *r3.FieldSpec) TOMLField {
	if f == nil {
		return ""
	}
	return TOMLField(f.String())
}

// TOMLToField converts a TOMLField to a FieldSpec.
func TOMLToField(tf TOMLField) *r3.FieldSpec {
	return r3.NewFieldSpec(tf.String())
}

// FilterToTOML converts a FilterSpec to a TOMLFilter. Returns ErrTOMLNullValue
// for a nil filter value, since TOML has no null type.
func FilterToTOML(f *r3.FilterSpec) (*TOMLFilter, error) {
	if f == nil {
		return nil, newError(errors.New("nil filter spec"))
	}

	// Relationship ("has") filters resolve in the driver and have no serialized
	// form; fail loudly rather than silently emit a match-all filter.
	if f.Relation != "" {
		return nil, newError(fmt.Errorf("relationship filter on %q cannot be serialized to TOML", f.Relation))
	}

	tf := &TOMLFilter{}

	if f.Field != nil {
		// Reject nil values, except value-less operators like "exists" that carry
		// no value and must round-trip (other dialects encode them fine).
		if f.Value == nil && !isValuelessOperator(f.Operator) {
			return nil, newError(fmt.Errorf("%w: field %q with operator %v",
				ErrTOMLNullValue, f.Field.String(), f.Operator))
		}

		tf.Field = f.Field.String()
		tf.Value = f.Value

		name := canonical.FormatFilterOperator(f.Operator)
		if name == "" {
			return nil, newError(fmt.Errorf("unsupported operator: %v", f.Operator))
		}
		tf.Operator = name
	}

	if len(f.And) > 0 {
		tf.And = make([]*TOMLFilter, len(f.And))
		for i, child := range f.And {
			cf, err := FilterToTOML(child)
			if err != nil {
				return nil, err
			}
			tf.And[i] = cf
		}
	}

	if len(f.Or) > 0 {
		tf.Or = make([]*TOMLFilter, len(f.Or))
		for i, child := range f.Or {
			cf, err := FilterToTOML(child)
			if err != nil {
				return nil, err
			}
			tf.Or[i] = cf
		}
	}

	return tf, nil
}

// isValuelessOperator reports whether an operator carries no value and so is
// exempt from the TOML null-value guard. "exists" only tests field presence.
func isValuelessOperator(op r3.FilterOperatorSpec) bool {
	return op == r3.OperatorExists
}

// TOMLToFilter converts a TOMLFilter to a FilterSpec.
func TOMLToFilter(tf *TOMLFilter) (*r3.FilterSpec, error) {
	return tf.ToFilterSpec()
}

// TOMLToFilters converts a slice of TOMLFilters to r3.Filters.
func TOMLToFilters(tfs TOMLFilters) (r3.Filters, error) {
	return tfs.toFilters()
}

// SortToTOML converts a SortSpec to a TOMLSort.
func SortToTOML(s *r3.SortSpec) *TOMLSort {
	if s == nil {
		return nil
	}
	return &TOMLSort{
		Field:         s.Column.String(),
		Direction:     canonical.FormatSortDirection(s.Direction),
		NullsPosition: canonical.FormatNullsPosition(s.NullsPosition),
	}
}

// TOMLToSort converts a TOMLSort to a SortSpec.
func TOMLToSort(ts *TOMLSort) (*r3.SortSpec, error) {
	return ts.ToSortSpec()
}

// PaginationToTOML converts a PaginationSpec to a TOMLPagination.
func PaginationToTOML(p *r3.PaginationSpec) *TOMLPagination {
	if p == nil {
		return &TOMLPagination{}
	}
	return &TOMLPagination{
		PageNum:  p.GetPageNum(),
		PageSize: p.GetPageSize(),
	}
}

// TOMLToPagination converts a TOMLPagination to a PaginationSpec.
func TOMLToPagination(tp *TOMLPagination) (*r3.PaginationSpec, error) {
	return tp.ToPaginationSpec()
}
