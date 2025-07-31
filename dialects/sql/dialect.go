package r3sql

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/k1/quick"
	"github.com/amberpixels/r3"
)

// SQLDialector is an outbound r3 dialector for SQL.
type SQLDialector struct{}

var (
	_ r3.FieldOutboundDialector          = (*SQLDialector)(nil)
	_ r3.FilterOutboundDialector         = (*SQLDialector)(nil)
	_ r3.FilterOperatorOutboundDialector = (*SQLDialector)(nil)
)

// ToSQLColumn converts a FieldSpec to an SQLColumn.
func (sv *SQLDialector) ToSQLColumn(cf *r3.FieldSpec) SQLColumn {
	return SQLColumn(cf.String())
}

// TranslateFieldSpec converts a FieldSpec to a DialectValue.
func (sv *SQLDialector) TranslateFieldSpec(cf *r3.FieldSpec) (r3.DialectValue, error) {
	return sv.ToSQLColumn(cf), nil
}

// TranslateFilterSpec converts a FilterSpec to a DialectValue.
func (sv *SQLDialector) TranslateFilterSpec(cf *r3.FilterSpec) (r3.DialectValue, error) {
	// Case 1. A simple filter when Field is set.

	if cf.Field != nil {
		sqlCol := sv.ToSQLColumn(cf.Field)

		// Extract join information from the field.
		joins := extractJoinFromField(sqlCol)
		// Nil value means we need to generate an "IS NULL" or "IS NOT NULL" clause.
		if cf.Value == nil {
			if cf.Operator == r3.OperatorEq { //nolint: staticcheck // we care only about these 2 values here
				clause := fmt.Sprintf("%s IS NULL", cf.Field)
				return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
			} else if cf.Operator == r3.OperatorNe {
				clause := fmt.Sprintf("%s IS NOT NULL", cf.Field)
				return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
			}

			return nil, fmt.Errorf("unsupported operator %q for nil value", cf.Operator)
		}
		// Simple case: Field is set and Value is non-nil.
		// Use our helper to convert the operator to its dialect string.
		opDialect, err := cf.Operator.ToDialect(sv)
		if err != nil {
			return nil, err
		}
		sqlClauseOperator, ok := opDialect.(SQLClauseOperator)
		if !ok {
			return nil, errors.New("unexpected type returned from SQLClauseOperator conversion")
		}

		clause := fmt.Sprintf("%s %s ?", cf.Field, sqlClauseOperator)
		return SQLClause{
			Clause: clause,
			Args:   []any{cf.Value},
			Joins:  joins,
		}, nil
	}

	// Case 2. A compound filter (logical group).
	// We assume that when Field is empty, either And or Or is non-empty.
	var children r3.Filters
	logicalOp := "AND"
	if len(cf.Or) > 0 {
		children = cf.Or
		logicalOp = "OR"
	} else {
		children = cf.And
	}

	var conditions []string
	var args []any
	var joins []SQLColumn

	// Process each child filter recursively.
	for _, child := range children {
		childFilter, ok := child.(*r3.FilterSpec)
		if !ok {
			return nil, errors.New("unexpected type returned from child filter conversion")
		}
		result, err := sv.TranslateFilterSpec(childFilter)
		if err != nil {
			return nil, err
		}
		childClause, ok := result.(SQLClause)
		if !ok {
			return nil, errors.New("unexpected type returned from child filter conversion")
		}
		conditions = append(conditions, childClause.Clause)
		args = append(args, childClause.Args...)
		joins = quick.Append(joins, childClause.Joins...)
	}

	// Combine the child conditions with the logical operator.
	combined := "(" + strings.Join(conditions, " "+logicalOp+" ") + ")"
	return SQLClause{
		Clause: combined,
		Args:   args,
		Joins:  joins,
	}, nil
}

// TranslateFilterOperatorSpec converts a FilterOperatorSpec to an DialectValue.
func (sv *SQLDialector) TranslateFilterOperatorSpec(op *r3.FilterOperatorSpec) (r3.DialectValue, error) {
	if op == nil {
		return nil, errors.New("nil filter operator")
	}

	switch *op {
	case r3.OperatorEq:
		return SQLClauseOperatorEq, nil
	case r3.OperatorNe:
		return SQLClauseOperatorNe, nil
	case r3.OperatorGt:
		return SQLClauseOperatorGt, nil
	case r3.OperatorGte:
		return SQLClauseOperatorGte, nil
	case r3.OperatorLt:
		return SQLClauseOperatorLt, nil
	case r3.OperatorLte:
		return SQLClauseOperatorLte, nil
	case r3.OperatorIn:
		return SQLClauseOperatorIn, nil
	case r3.OperatorNotIn:
		return SQLClauseOperatorNotIn, nil
	case r3.OperatorLike:
		return SQLClauseOperatorLike, nil
	case r3.OperatorNotLike:
		return SQLClauseOperatorNotLike, nil
	case r3.OperatorILike:
		return SQLClauseOperatorILike, nil

	case r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx,
		r3.OperatorExists:
		return nil, fmt.Errorf("not implemented: %s", op)

	case r3.OperatorUnspecified:
		fallthrough
	default:
		return nil, fmt.Errorf("unsupported filter operator: %s", op)
	}
}
