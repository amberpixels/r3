package r3sql

import (
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/internal/gx"
)

// SQLClause represents a r3.Filter that can be converted into SQL Clause (used by gorm, sqlx, squirrel, etc.)
type SQLClause struct {
	// Clause is the raw safe SQL string (e.g. "name = ?")
	Clause string

	// Args is a slice of arguments to be bound to the SQL clause.
	// Len of arguments must match the number of placeholders in the clause.
	Args []any

	// Joins is a slice of joins that are required for the clause.
	Joins []string
}

type SQLClauses []SQLClause

func (cs SQLClauses) Joins() []string {
	var joins []string
	for _, c := range cs {
		joins = gx.Qappend(joins, c.Joins...)
	}
	return joins
}

// SqlDialector is a visitor for r3.Filter that converts it to an SQLClause.
type SqlDialector struct{}

var _ r3.FilterDialector = (*SqlDialector)(nil)

// FromColumnFilter converts a ColumnFilter to an SQLClause, handling both simple and group filters.
func (sv *SqlDialector) FromColumnFilter(cf *r3.ColumnFilter) (r3.DialectValue, error) {
	// Case 1. A simple filter when Field is set.
	fieldRaw, err := cf.Field.ToDialect(sv)
	if err != nil {
		return nil, err
	}
	field, ok := fieldRaw.(ColumnField)
	if !ok {
		return nil, fmt.Errorf("unexpected field type: %T", fieldRaw)
	}

	if field != "" {
		// Extract join information from the field.
		joins := extractJoinFromField(field)
		// Nil value means we need to generate an "IS NULL" or "IS NOT NULL" clause.
		if cf.Value == nil {
			switch cf.Operator {
			case r3.OperatorEq:
				clause := fmt.Sprintf("%s IS NULL", cf.Field)
				return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
			case r3.OperatorNe:
				clause := fmt.Sprintf("%s IS NOT NULL", cf.Field)
				return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
			default:
				return nil, fmt.Errorf("unsupported operator %q for nil value", cf.Operator)
			}
		}
		// Simple case: Field is set and Value is non-nil.
		// Use our helper to convert the operator to its dialect string.
		opDialect, err := cf.Operator.ToDialect(&SQLOperatorDialector{})
		if err != nil {
			return nil, err
		}

		clause := fmt.Sprintf("%s %s ?", cf.Field, opDialect.(SQLClauseOperator))
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
	var joins []string

	// Process each child filter recursively.
	for _, child := range children {
		result, err := sv.FromColumnFilter(child.(*r3.ColumnFilter))
		if err != nil {
			return nil, err
		}
		childClause, ok := result.(SQLClause)
		if !ok {
			return nil, fmt.Errorf("unexpected type returned from child filter conversion")
		}
		conditions = append(conditions, childClause.Clause)
		args = append(args, childClause.Args...)
		joins = gx.Qappend(joins, childClause.Joins...)
	}

	// Combine the child conditions with the logical operator.
	combined := "(" + strings.Join(conditions, " "+logicalOp+" ") + ")"
	return SQLClause{
		Clause: combined,
		Args:   args,
		Joins:  joins,
	}, nil
}

func (sv *SqlDialector) FromFilters(filters r3.Filters) (SQLClauses, error) {
	if len(filters) == 0 {
		return nil, nil
	}

	result := make(SQLClauses, len(filters))
	for i, f := range filters {
		clauseRaw, err := f.ToDialect(sv)
		if err != nil {
			return nil, fmt.Errorf("sql dialector failed: %w", err)
		}

		clause, ok := clauseRaw.(SQLClause)
		if !ok {
			return nil, fmt.Errorf("sql dialector returned unexpected result type")
		}

		result[i] = clause
	}

	return result, nil
}

func FiltersToSQLClauses(filters r3.Filters) (SQLClauses, error) {
	sv := &SqlDialector{}
	return sv.FromFilters(filters)
}

// extractJoinFromField inspects the field name and returns a slice with join information if necessary.
// For demonstration, assume join information is determined by a prefix separated by a dot.
// E.g., "user.name" could signal a join to the "user" table.
func extractJoinFromField(field ColumnField) []string {
	// If there is a dot, we assume the portion before the dot indicates a join.
	parts := strings.Split(string(field), ".")
	if len(parts) > 1 && parts[0] != "" {
		return []string{parts[0]}
	}
	return nil
}
