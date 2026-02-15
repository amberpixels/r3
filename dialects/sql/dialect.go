package r3sql

// TODO: Dialect Flavors
//
// Dialects can have flavors to handle database-specific SQL variations.
// When a driver selects a dialect, it should also be able to select the
// proper flavor of that dialect. For example:
//
//   - PostgreSQL flavor: supports ILIKE, uses $1/$2 placeholders, supports RETURNING
//   - SQLite flavor: maps ILIKE -> LIKE (SQLite LIKE is case-insensitive for ASCII),
//     uses ? placeholders, RETURNING requires SQLite 3.35+
//   - MySQL flavor: maps ILIKE -> LIKE (with collation-dependent behavior),
//     uses ? placeholders, no NULLS FIRST/LAST (needs CASE WHEN workaround)
//
// Proposed API:
//
//   type Flavor int
//   const (
//       FlavorPostgreSQL Flavor = iota
//       FlavorSQLite
//       FlavorMySQL
//   )
//
//   // Option funcs for dialect conversion:
//   FiltersToSQL(filters, WithFlavor(FlavorSQLite))
//   SortsToSQL(sorts, WithFlavor(FlavorSQLite))
//
// This keeps the dialect package as the single SQL translation layer,
// with drivers only needing to specify which flavor they require.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/amberpixels/k1/quick"
	"github.com/amberpixels/r3"
)

// FieldToSQL converts a FieldSpec to an SQLColumn (raw, no quoting).
// This does NOT validate or quote the identifier. For safe SQL generation from
// user-supplied field names, use SafeColumnExpr instead.
func FieldToSQL(f *r3.FieldSpec) SQLColumn {
	return SQLColumn(f.String())
}

// SQLToField converts an SQLColumn to a FieldSpec.
func SQLToField(col SQLColumn) *r3.FieldSpec {
	return r3.NewFieldSpec(col.String())
}

// OperatorToSQL converts a FilterOperatorSpec to an SQLClauseOperator.
func OperatorToSQL(op r3.FilterOperatorSpec) (SQLClauseOperator, error) {
	switch op {
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
		// TODO: With dialect flavors, this should respect the selected flavor:
		//   PostgreSQL -> ILIKE (native)
		//   SQLite     -> LIKE  (SQLite LIKE is case-insensitive for ASCII)
		//   MySQL      -> LIKE  (case sensitivity depends on column collation)
		return SQLClauseOperatorILike, nil

	case r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx,
		r3.OperatorExists:
		return "", fmt.Errorf("not implemented: %s", &op)

	case r3.OperatorUnspecified:
		fallthrough
	default:
		return "", fmt.Errorf("unsupported filter operator: %s", &op)
	}
}

// SQLToOperator converts an SQLClauseOperator back to a FilterOperatorSpec.
func SQLToOperator(op SQLClauseOperator) (r3.FilterOperatorSpec, error) {
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

// FilterToSQL converts a FilterSpec to an SQLClause.
func FilterToSQL(cf *r3.FilterSpec) (SQLClause, error) {
	// Case 1. A simple filter when Field is set.
	if cf.Field != nil {
		// Validate and quote the field name to prevent SQL injection.
		safeCol, err := SafeColumnExpr(cf.Field)
		if err != nil {
			return SQLClause{}, fmt.Errorf("unsafe filter field: %w", err)
		}

		// Extract join information from the field (uses raw name for table extraction).
		joins, err := extractJoinFromField(cf.Field.String())
		if err != nil {
			return SQLClause{}, fmt.Errorf("unsafe join in filter field: %w", err)
		}

		// Nil value means we need to generate an "IS NULL" or "IS NOT NULL" clause.
		if cf.Value == nil {
			if cf.Operator == r3.OperatorEq { //nolint: staticcheck // we care only about these 2 values here
				clause := fmt.Sprintf("%s %s", safeCol, sqlIsNull)
				return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
			} else if cf.Operator == r3.OperatorNe {
				clause := fmt.Sprintf("%s %s", safeCol, sqlIsNotNull)
				return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
			}

			return SQLClause{}, fmt.Errorf("unsupported operator %q for nil value", cf.Operator)
		}

		// Simple case: Field is set and Value is non-nil.
		sqlClauseOperator, err := OperatorToSQL(cf.Operator)
		if err != nil {
			return SQLClause{}, err
		}

		clause := fmt.Sprintf("%s %s ?", safeCol, sqlClauseOperator)
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
		childClause, err := FilterToSQL(child)
		if err != nil {
			return SQLClause{}, fmt.Errorf("failed to translate child filter: %w", err)
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

// FiltersToSQL translates a list of r3.Filters into SQLClauses.
func FiltersToSQL(filters r3.Filters) (SQLClauses, error) {
	if len(filters) == 0 {
		return SQLClauses{}, nil
	}

	result := make(SQLClauses, len(filters))
	for i, f := range filters {
		clause, err := FilterToSQL(f)
		if err != nil {
			return nil, newError(err)
		}
		result[i] = clause
	}

	return result, nil
}

// FiltersToSQLClauses is an alias for FiltersToSQL for backward compatibility.
//
// Deprecated: Use FiltersToSQL instead.
func FiltersToSQLClauses(filters r3.Filters) (SQLClauses, error) {
	return FiltersToSQL(filters)
}

// SortToSQL converts a SortSpec to an SQLSort.
func SortToSQL(s *r3.SortSpec) (SQLSort, error) {
	if s == nil {
		return "", errors.New("sort spec cannot be nil")
	}

	// Validate and quote the column name to prevent SQL injection.
	safeCol, err := SafeColumnExpr(s.Column)
	if err != nil {
		return "", fmt.Errorf("unsafe sort column: %w", err)
	}

	// Build SQL sort string
	sortStr := safeCol + " " + sqlSortDirectionString(s.Direction)

	if s.NullsPosition != r3.NullsPositionNotSpecified {
		sortStr += " " + sqlNullsPositionString(s.NullsPosition)
	}

	return SQLSort(sortStr), nil
}

// SortsToSQL converts a slice of SortSpec to a slice of SQLSort.
func SortsToSQL(ss r3.Sorts) ([]SQLSort, error) {
	result := make([]SQLSort, len(ss))
	for i, s := range ss {
		sqlSort, err := SortToSQL(s)
		if err != nil {
			return nil, err
		}
		result[i] = sqlSort
	}
	return result, nil
}

// SQLToSort converts an SQLSort string back to a SortSpec.
func SQLToSort(s SQLSort) (*r3.SortSpec, error) {
	sortStr := string(s)
	return parseSQLSortString(sortStr)
}

// PaginationToSQL converts a PaginationSpec to SQLPagination.
func PaginationToSQL(p *r3.PaginationSpec) *SQLPagination {
	if p == nil {
		return nil
	}
	limit, offset := p.ToLimitOffset()
	return NewSQLPagination(limit, offset)
}

// SQLToPagination converts SQLPagination to PaginationSpec.
func SQLToPagination(p *SQLPagination) *r3.PaginationSpec {
	if p == nil {
		return nil
	}

	// Convert limit/offset back to page number/page size
	// Offset = (PageNum - 1) * PageSize, so PageNum = (Offset / PageSize) + 1
	pageSize := p.Limit
	pageNum := 1 // Default to page 1
	if pageSize > 0 && p.Offset > 0 {
		pageNum = (p.Offset / pageSize) + 1
	}

	return r3.NewPaginationSpec(pageNum, pageSize)
}

// --- Internal helpers ---

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
