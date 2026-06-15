package r3sql

// NOTE: Database-specific SQL variations (ILIKE behavior, placeholder style,
// NULLS FIRST/LAST support) are handled at the engine layer via engine/sql.Flavor,
// not at the dialect level. The dialect package generates generic SQL that works
// across databases, while Flavor handles DB-specific placeholder conversion
// and query construction.

import (
	"errors"
	"fmt"
	"reflect"
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
		// Generates ILIKE which is PostgreSQL-native. For MySQL/SQLite compatibility,
		// the engine/sql.Flavor layer handles any necessary conversion.
		return SQLClauseOperatorILike, nil

	case r3.OperatorExists:
		// Exists is handled directly in FilterToSQL as IS NOT NULL.
		return "", errors.New("exists operator must be handled via FilterToSQL, not OperatorToSQL")

	case r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx:
		// Between operators are handled directly in FilterToSQL as compound conditions.
		return "", errors.New("between operators must be handled via FilterToSQL, not OperatorToSQL")

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

			return SQLClause{}, fmt.Errorf("unsupported operator %v for nil value", cf.Operator)
		}

		// Exists: translate to IS NOT NULL
		if cf.Operator == r3.OperatorExists {
			clause := fmt.Sprintf("%s %s", safeCol, sqlIsNotNull)
			return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
		}

		// Between operators: value must be a 2-element slice [low, high].
		if isBetweenOperator(cf.Operator) {
			return betweenToSQL(safeCol, cf.Operator, cf.Value, joins)
		}

		// IN / NOT IN: the value is a set, so it needs one placeholder per
		// element ("col IN (?, ?, ?)"). A single "col IN ?" placeholder bound to
		// a slice is NOT expanded by database/sql, so it must be expanded here.
		if cf.Operator == r3.OperatorIn || cf.Operator == r3.OperatorNotIn {
			sqlOp, err := OperatorToSQL(cf.Operator)
			if err != nil {
				return SQLClause{}, err
			}
			return inToSQL(safeCol, sqlOp, cf.Value, joins), nil
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
		col := r3.FieldSpec(unquoteSQLColumn(sortStr))
		return &r3.SortSpec{Column: &col}, nil
	}

	parts := strings.Split(sortStr, " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid sort format: %s", sortStr)
	}

	// Parse field. SortToSQL quotes the column (e.g. `"name"`, `"user"."name"`),
	// so strip the quoting to recover the original field name. Identifiers never
	// contain spaces, so the column is always a single space-delimited token.
	col := r3.FieldSpec(unquoteSQLColumn(parts[0]))

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

// isBetweenOperator returns true if the operator is any of the between variants.
func isBetweenOperator(op r3.FilterOperatorSpec) bool {
	//nolint:exhaustive // only checking between variants
	switch op {
	case r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx:
		return true
	default:
		return false
	}
}

// betweenToSQL converts a between filter to a compound SQL condition.
// Value must be a 2-element slice: [low, high].
// Between variants:
//   - Between (inclusive both):              col >= ? AND col <= ?
//   - BetweenEx (exclusive both):            col > ? AND col < ?
//   - BetweenExInc (excl low, incl high):    col > ? AND col <= ?
//   - BetweenIncEx (incl low, excl high):    col >= ? AND col < ?
func betweenToSQL(safeCol string, op r3.FilterOperatorSpec, value any, joins []SQLColumn) (SQLClause, error) {
	low, high, err := r3.ExtractBetweenBounds(value)
	if err != nil {
		return SQLClause{}, err
	}

	var lowOp, highOp SQLClauseOperator
	//nolint:exhaustive // only between variants reach here, guarded by isBetweenOperator
	switch op {
	case r3.OperatorBetween:
		lowOp, highOp = SQLClauseOperatorGte, SQLClauseOperatorLte
	case r3.OperatorBetweenEx:
		lowOp, highOp = SQLClauseOperatorGt, SQLClauseOperatorLt
	case r3.OperatorBetweenExInc:
		lowOp, highOp = SQLClauseOperatorGt, SQLClauseOperatorLte
	case r3.OperatorBetweenIncEx:
		lowOp, highOp = SQLClauseOperatorGte, SQLClauseOperatorLt
	default:
		return SQLClause{}, fmt.Errorf("unexpected between operator: %v", op)
	}

	clause := fmt.Sprintf("(%s %s ? %s %s %s ?)", safeCol, lowOp, sqlAnd, safeCol, highOp)
	return SQLClause{
		Clause: clause,
		Args:   []any{low, high},
		Joins:  joins,
	}, nil
}

// inToSQL converts an IN / NOT IN filter into a clause with one placeholder per
// value: "col IN (?, ?, ?)". The values are flattened into Args so that both the
// raw database/sql engine and the ORM drivers (which forward Clause + Args to
// their query builders) bind them as individual positional arguments.
//
// An empty set collapses to a constant predicate, since "col IN ()" is invalid
// SQL: "col IN ()" matches nothing, "col NOT IN ()" matches everything.
func inToSQL(safeCol string, op SQLClauseOperator, value any, joins []SQLColumn) SQLClause {
	args := flattenInValues(value)
	if len(args) == 0 {
		predicate := sqlFalse
		if op == SQLClauseOperatorNotIn {
			predicate = sqlTrue
		}
		// The join is only needed to reference the column; a constant predicate
		// references nothing, so it is dropped.
		return SQLClause{Clause: predicate, Args: nil, Joins: nil}
	}

	placeholders := strings.Repeat("?, ", len(args)-1) + "?"
	clause := fmt.Sprintf("%s %s (%s)", safeCol, op, placeholders)
	return SQLClause{Clause: clause, Args: args, Joins: joins}
}

// flattenInValues expands an IN value into individual arguments. A slice or
// array yields one argument per element; any other value (including a scalar
// passed to In) yields a single-element set. A []byte is treated as one scalar
// value (a BLOB), not a list of bytes, matching how SQL drivers handle it.
func flattenInValues(value any) []any {
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return []any{value}
		}
		args := make([]any, rv.Len())
		for i := range args {
			args[i] = rv.Index(i).Interface()
		}
		return args
	default:
		return []any{value}
	}
}
