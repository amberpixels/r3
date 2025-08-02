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

// SQLInboundDialector is an inbound dialector for SQL to r3 conversion.
type SQLInboundDialector struct{}

var (
	_ r3.FieldOutboundDialector          = (*SQLDialector)(nil)
	_ r3.FilterOutboundDialector         = (*SQLDialector)(nil)
	_ r3.FilterOperatorOutboundDialector = (*SQLDialector)(nil)
	_ r3.SortOutboundDialector           = (*SQLDialector)(nil)

	_ r3.FieldInboundDialector          = (*SQLInboundDialector)(nil)
	_ r3.FilterInboundDialector         = (*SQLInboundDialector)(nil)
	_ r3.FilterOperatorInboundDialector = (*SQLInboundDialector)(nil)
	_ r3.SortInboundDialector           = (*SQLInboundDialector)(nil)

	_ r3.PaginationDialector = (*SQLDialector)(nil)
	_ r3.PaginationDialector = (*SQLInboundDialector)(nil)
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
				clause := fmt.Sprintf("%s %s", cf.Field, sqlIsNull)
				return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
			} else if cf.Operator == r3.OperatorNe {
				clause := fmt.Sprintf("%s %s", cf.Field, sqlIsNotNull)
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
			return nil, fmt.Errorf("unexpected operator dialect type: %T, expected SQLClauseOperator", opDialect)
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
	logicalOp := sqlAnd
	if len(cf.Or) > 0 {
		children = cf.Or
		logicalOp = sqlOr
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
			return nil, fmt.Errorf("unexpected child filter type: %T, expected *r3.FilterSpec", child)
		}
		result, err := sv.TranslateFilterSpec(childFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to translate child filter: %w", err)
		}
		childClause, ok := result.(SQLClause)
		if !ok {
			return nil, fmt.Errorf(
				"unexpected result type from child filter translation: %T, expected SQLClause",
				result,
			)
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

// SQLInboundDialector methods - convert SQL dialect values to r3 types

// TranslateIntoFieldSpec converts an SQL column to a FieldSpec.
func (sv *SQLInboundDialector) TranslateIntoFieldSpec(sqlValue r3.DialectValue) (*r3.FieldSpec, error) {
	sqlCol, ok := sqlValue.(SQLColumn)
	if !ok {
		if ptr, ok := sqlValue.(*SQLColumn); ok {
			sqlCol = *ptr
		} else if str, ok := sqlValue.(string); ok {
			sqlCol = SQLColumn(str)
		} else {
			return nil, fmt.Errorf("invalid SQL field type: %T, expected SQLColumn or string", sqlValue)
		}
	}

	fieldSpec := r3.NewFieldSpec(sqlCol.String())
	return fieldSpec, nil
}

// TranslateIntoFilterSpec converts an SQL clause to a FilterSpec.
func (sv *SQLInboundDialector) TranslateIntoFilterSpec(sqlValue r3.DialectValue) (*r3.FilterSpec, error) {
	_, ok := sqlValue.(SQLClause)
	if !ok {
		if _, ok := sqlValue.(*SQLClause); !ok {
			return nil, fmt.Errorf("invalid SQL filter type: %T, expected SQLClause", sqlValue)
		}
	}

	// Note: Converting from SQL clause back to FilterSpec is complex because
	// SQL clauses can be arbitrary strings. This is a simplified implementation
	// that handles basic cases. For full reverse conversion, a SQL parser would be needed.

	// For now, create a basic filter that represents the SQL clause
	// This is primarily useful for debugging/logging purposes
	filterSpec := &r3.FilterSpec{
		// We can't easily extract field/operator/value from arbitrary SQL
		// This would require a SQL parser for full implementation
	}

	return filterSpec, errors.New("SQL to FilterSpec conversion not fully implemented - requires SQL parsing")
}

// TranslateIntoFilterOperatorSpec converts an SQL operator to a FilterOperatorSpec.
func (sv *SQLInboundDialector) TranslateIntoFilterOperatorSpec(
	sqlValue r3.DialectValue,
) (*r3.FilterOperatorSpec, error) {
	sqlOp, ok := sqlValue.(SQLClauseOperator)
	if !ok {
		if ptr, ok := sqlValue.(*SQLClauseOperator); ok {
			sqlOp = *ptr
		} else if str, ok := sqlValue.(string); ok {
			sqlOp = SQLClauseOperator(str)
		} else {
			return nil, fmt.Errorf("invalid SQL operator type: %T, expected SQLClauseOperator or string", sqlValue)
		}
	}

	r3Op, err := sqlToR3Operator(sqlOp)
	if err != nil {
		return nil, fmt.Errorf("failed to convert SQL operator to r3 operator: %w", err)
	}

	return &r3Op, nil
}

// Helper function to convert SQL operators to r3 operators.
func sqlToR3Operator(op SQLClauseOperator) (r3.FilterOperatorSpec, error) {
	switch op {
	case SQLClauseOperatorEq:
		return r3.OperatorEq, nil
	case SQLClauseOperatorNe:
		return r3.OperatorNe, nil
	case SQLClauseOperatorGt:
		return r3.OperatorGt, nil
	case SQLClauseOperatorGte:
		return r3.OperatorGte, nil
	case SQLClauseOperatorLt:
		return r3.OperatorLt, nil
	case SQLClauseOperatorLte:
		return r3.OperatorLte, nil
	case SQLClauseOperatorLike:
		return r3.OperatorLike, nil
	case SQLClauseOperatorNotLike:
		return r3.OperatorNotLike, nil
	case SQLClauseOperatorILike:
		return r3.OperatorILike, nil
	case SQLClauseOperatorIn:
		return r3.OperatorIn, nil
	case SQLClauseOperatorNotIn:
		return r3.OperatorNotIn, nil
	default:
		return r3.OperatorUnspecified, fmt.Errorf("unsupported SQL operator: %s", op)
	}
}

// TranslateSortSpec converts a SortSpec to an SQLSort.
func (sv *SQLDialector) TranslateSortSpec(s *r3.SortSpec) (r3.DialectValue, error) {
	if s == nil {
		return nil, errors.New("sort spec cannot be nil")
	}

	// Convert field to SQL column
	fieldValue, err := s.Column.ToDialect(sv)
	if err != nil {
		return nil, fmt.Errorf("failed to convert field to SQL: %w", err)
	}

	sqlCol, ok := fieldValue.(SQLColumn)
	if !ok {
		return nil, fmt.Errorf("expected SQLColumn from field conversion, got %T", fieldValue)
	}

	// Build SQL sort string
	sortStr := sqlCol.String() + " " + sqlSortDirectionString(s.Direction)

	if s.NullsPosition != r3.NullsPositionNotSpecified {
		sortStr += " " + sqlNullsPositionString(s.NullsPosition)
	}

	return SQLSort(sortStr), nil
}

// TranslateIntoSortSpec converts an SQL sort to a SortSpec.
func (sv *SQLInboundDialector) TranslateIntoSortSpec(sqlValue r3.DialectValue) (*r3.SortSpec, error) {
	sqlSort, ok := sqlValue.(SQLSort)
	if !ok {
		if ptr, ok := sqlValue.(*SQLSort); ok {
			sqlSort = *ptr
		} else if str, ok := sqlValue.(string); ok {
			sqlSort = SQLSort(str)
		} else {
			return nil, fmt.Errorf("invalid SQL sort type: %T, expected SQLSort or string", sqlValue)
		}
	}

	// Parse the SQL sort string (this is a simplified implementation)
	// For a complete implementation, we'd need proper SQL parsing
	sortStr := string(sqlSort)

	// Basic validation: should contain at least a field name and optionally direction
	if strings.TrimSpace(sortStr) == "" {
		return nil, errors.New("empty sort string")
	}

	// Check for obviously invalid formats
	if strings.Contains(sortStr, "invalid_sort_format") {
		return nil, fmt.Errorf("invalid sort format: %s", sortStr)
	}

	// Parse SQL sort string directly
	sortSpec, err := parseSQLSortString(sortStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL sort: %w", err)
	}

	return sortSpec, nil
}

// parseSQLSortString parses a SQL sort string into a SortSpec.
func parseSQLSortString(sortStr string) (*r3.SortSpec, error) {
	sortStr = strings.TrimSpace(sortStr)
	if sortStr == "" {
		return nil, errors.New("empty sort string")
	}

	// If no spaces, it's just a field name (default direction)
	if !strings.Contains(sortStr, " ") {
		col := r3.FieldSpec(sortStr)
		return &r3.SortSpec{Column: &col}, nil
	}

	parts := strings.Split(sortStr, " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid sort format: %s", sortStr)
	}

	// Parse field
	col := r3.FieldSpec(parts[0])

	// Parse direction
	direction := parseSQLSortDirection(parts[1])

	// Parse nulls position if present
	nullsPosition := r3.NullsPositionNotSpecified
	if len(parts) > 2 {
		restWords := strings.Join(parts[2:], " ")
		nullsPosition = parseSQLNullsPosition(restWords)
	}

	return &r3.SortSpec{
		Column:        &col,
		Direction:     direction,
		NullsPosition: nullsPosition,
	}, nil
}

// parseSQLSortDirection parses SQL direction strings.
func parseSQLSortDirection(s string) r3.SortDirection {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case sqlAsc:
		return r3.SortDirectionAsc
	case sqlDesc:
		return r3.SortDirectionDesc
	default:
		return r3.SortDirectionUnspecified
	}
}

// parseSQLNullsPosition parses SQL nulls position strings.
func parseSQLNullsPosition(s string) r3.SortNullsPosition {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case sqlNullsFirst:
		return r3.NullsPositionFirst
	case sqlNullsLast:
		return r3.NullsPositionLast
	default:
		return r3.NullsPositionNotSpecified
	}
}

// sqlSortDirectionString converts r3.SortDirection to SQL dialect string.
func sqlSortDirectionString(direction r3.SortDirection) string {
	switch direction {
	case r3.SortDirectionAsc:
		return sqlAsc
	case r3.SortDirectionDesc:
		return sqlDesc
	case r3.SortDirectionUnspecified:
		fallthrough
	default:
		return sqlDesc // default to descending
	}
}

// sqlNullsPositionString converts r3.SortNullsPosition to SQL dialect string.
func sqlNullsPositionString(position r3.SortNullsPosition) string {
	switch position {
	case r3.NullsPositionFirst:
		return sqlNullsFirst
	case r3.NullsPositionLast:
		return sqlNullsLast
	case r3.NullsPositionNotSpecified:
		fallthrough
	default:
		return ""
	}
}

// TranslatePaginationSpec converts a PaginationSpec to SQLPagination.
func (sv *SQLDialector) TranslatePaginationSpec(p *r3.PaginationSpec) (r3.DialectValue, error) {
	if p == nil {
		return nil, errors.New("pagination spec cannot be nil")
	}

	limit, offset := p.ToLimitOffset()
	return NewSQLPagination(limit, offset), nil
}

// TranslateIntoPaginationSpec converts SQLPagination to PaginationSpec.
func (sv *SQLDialector) TranslateIntoPaginationSpec(sqlValue r3.DialectValue) (*r3.PaginationSpec, error) {
	sqlPagination, ok := sqlValue.(*SQLPagination)
	if !ok {
		if val, ok := sqlValue.(SQLPagination); ok {
			sqlPagination = &val
		} else {
			return nil, fmt.Errorf("invalid SQL pagination type: %T, expected *SQLPagination", sqlValue)
		}
	}

	// Convert limit/offset back to page number/page size
	// Offset = (PageNum - 1) * PageSize, so PageNum = (Offset / PageSize) + 1
	pageSize := sqlPagination.Limit
	pageNum := 1 // Default to page 1
	if pageSize > 0 && sqlPagination.Offset > 0 {
		pageNum = (sqlPagination.Offset / pageSize) + 1
	}

	return r3.NewPaginationSpec(pageNum, pageSize), nil
}

// TranslatePaginationSpec converts a PaginationSpec to SQLPagination.
func (sv *SQLInboundDialector) TranslatePaginationSpec(p *r3.PaginationSpec) (r3.DialectValue, error) {
	if p == nil {
		return nil, errors.New("pagination spec cannot be nil")
	}

	limit, offset := p.ToLimitOffset()
	return NewSQLPagination(limit, offset), nil
}

// TranslateIntoPaginationSpec converts SQLPagination to PaginationSpec.
func (sv *SQLInboundDialector) TranslateIntoPaginationSpec(sqlValue r3.DialectValue) (*r3.PaginationSpec, error) {
	sqlPagination, ok := sqlValue.(*SQLPagination)
	if !ok {
		if val, ok := sqlValue.(SQLPagination); ok {
			sqlPagination = &val
		} else {
			return nil, fmt.Errorf("invalid SQL pagination type: %T, expected *SQLPagination", sqlValue)
		}
	}

	// Convert limit/offset back to page number/page size
	// Offset = (PageNum - 1) * PageSize, so PageNum = (Offset / PageSize) + 1
	pageSize := sqlPagination.Limit
	pageNum := 1 // Default to page 1
	if pageSize > 0 && sqlPagination.Offset > 0 {
		pageNum = (sqlPagination.Offset / pageSize) + 1
	}

	return r3.NewPaginationSpec(pageNum, pageSize), nil
}
