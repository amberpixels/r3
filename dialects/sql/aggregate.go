package r3sql

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

// AggregateExpr returns the bare, un-aliased SQL expression for an aggregate
// spec: COUNT(*), COUNT(DISTINCT "col"), SUM("col"), ... (field validated and
// quoted via SafeColumnExpr). HAVING must use this form, not the alias:
// PostgreSQL rejects select-list aliases in HAVING; expressions are portable.
func AggregateExpr(a *r3.AggregateSpec) (string, error) {
	if a == nil {
		return "", errors.New("nil aggregate spec")
	}

	if a.Func == r3.AggregateCount && (a.Field == nil || a.Field.String() == "") {
		return "COUNT(*)", nil
	}

	safeCol, err := SafeColumnExpr(a.Field)
	if err != nil {
		return "", fmt.Errorf("unsafe aggregate field: %w", err)
	}

	switch a.Func {
	case r3.AggregateCount:
		return "COUNT(" + safeCol + ")", nil
	case r3.AggregateCountDistinct:
		return "COUNT(DISTINCT " + safeCol + ")", nil
	case r3.AggregateSum:
		return "SUM(" + safeCol + ")", nil
	case r3.AggregateAvg:
		return "AVG(" + safeCol + ")", nil
	case r3.AggregateMin:
		return "MIN(" + safeCol + ")", nil
	case r3.AggregateMax:
		return "MAX(" + safeCol + ")", nil
	default:
		return "", fmt.Errorf("unsupported aggregate function: %v", a.Func)
	}
}

// AggregateToSQL returns the select-list item, `COUNT(*) AS "alias"`, with the
// alias validated and quoted.
func AggregateToSQL(a *r3.AggregateSpec) (string, error) {
	expr, err := AggregateExpr(a)
	if err != nil {
		return "", err
	}
	if err := r3.ValidateIdentifier(a.Alias); err != nil {
		return "", fmt.Errorf("unsafe aggregate alias: %w", err)
	}
	return expr + " AS " + QuoteIdentifier(a.Alias), nil
}

// GroupByToSQL validates and quotes the group-by fields, returning one quoted
// column expression per field.
func GroupByToSQL(fields r3.Fields) ([]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		safeCol, err := SafeColumnExpr(f)
		if err != nil {
			return nil, fmt.Errorf("unsafe group field: %w", err)
		}
		out = append(out, safeCol)
	}
	return out, nil
}

// HavingToSQL converts Having filters (implicitly ANDed) into one SQLClause.
// Each leaf's name resolves through exprByName - aggregate aliases to their full
// expressions (SUM("col")), group fields to quoted columns - because HAVING
// cannot reference select-list aliases portably. An absent name is an error
// (the caller validates first via r3.Schema.ValidateAggregateQuery).
func HavingToSQL(having r3.Filters, exprByName map[string]string) (SQLClause, error) {
	if len(having) == 0 {
		return SQLClause{}, nil
	}

	conditions := make([]string, 0, len(having))
	var args []any
	for _, f := range having {
		clause, err := havingFilterToSQL(f, exprByName)
		if err != nil {
			return SQLClause{}, newError(err)
		}
		conditions = append(conditions, clause.Clause)
		args = append(args, clause.Args...)
	}
	return SQLClause{Clause: strings.Join(conditions, " "+sqlAnd+" "), Args: args}, nil
}

// havingFilterToSQL mirrors FilterToSQL but resolves the column expression
// through exprByName instead of quoting the field name.
func havingFilterToSQL(f *r3.FilterSpec, exprByName map[string]string) (SQLClause, error) {
	if f == nil {
		return SQLClause{}, errors.New("nil having filter")
	}
	if f.Relation != "" {
		return SQLClause{}, fmt.Errorf("relationship filter %q has no meaning in HAVING", f.Relation)
	}

	if len(f.And) > 0 || len(f.Or) > 0 {
		children, logicalOp := f.And, sqlAnd
		if len(f.Or) > 0 {
			children, logicalOp = f.Or, sqlOr
		}
		conditions := make([]string, 0, len(children))
		var args []any
		for _, child := range children {
			clause, err := havingFilterToSQL(child, exprByName)
			if err != nil {
				return SQLClause{}, err
			}
			conditions = append(conditions, clause.Clause)
			args = append(args, clause.Args...)
		}
		return SQLClause{Clause: "(" + strings.Join(conditions, " "+logicalOp+" ") + ")", Args: args}, nil
	}

	if f.Field == nil {
		return SQLClause{}, errors.New("having filter with no field")
	}
	expr, ok := exprByName[f.Field.String()]
	if !ok {
		return SQLClause{}, fmt.Errorf("having references undeclared name %q", f.Field.String())
	}

	if f.Value == nil {
		switch f.Operator {
		case r3.OperatorEq:
			return SQLClause{Clause: expr + " " + sqlIsNull}, nil
		case r3.OperatorNe:
			return SQLClause{Clause: expr + " " + sqlIsNotNull}, nil
		default:
			return SQLClause{}, fmt.Errorf("unsupported operator %v for nil value in HAVING", f.Operator)
		}
	}

	if f.Operator == r3.OperatorExists {
		return SQLClause{Clause: expr + " " + sqlIsNotNull}, nil
	}

	if isBetweenOperator(f.Operator) {
		return betweenToSQL(expr, f.Operator, f.Value, nil)
	}

	if f.Operator == r3.OperatorIn || f.Operator == r3.OperatorNotIn {
		sqlOp, err := OperatorToSQL(f.Operator)
		if err != nil {
			return SQLClause{}, err
		}
		return inToSQL(expr, sqlOp, f.Value, nil), nil
	}

	sqlOp, err := OperatorToSQL(f.Operator)
	if err != nil {
		return SQLClause{}, err
	}
	return SQLClause{Clause: fmt.Sprintf("%s %s ?", expr, sqlOp), Args: []any{f.Value}}, nil
}
