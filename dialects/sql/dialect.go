package r3sql

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/amberpixels/k1/quick"

	"github.com/amberpixels/r3"
)

// FieldToSQL converts a FieldSpec to a raw, unquoted SQLColumn. It does NOT
// validate or quote; use SafeColumnExpr for user-supplied field names.
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
		// PostgreSQL-native ILIKE; engine/sql.Flavor converts it for MySQL/SQLite.
		return SQLClauseOperatorILike, nil

	case r3.OperatorExists:
		// Exists becomes IS NOT NULL, handled in FilterToSQL.
		return "", errors.New("exists operator must be handled via FilterToSQL, not OperatorToSQL")

	case r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx:
		// Between becomes a compound condition, handled in FilterToSQL.
		return "", errors.New("between operators must be handled via FilterToSQL, not OperatorToSQL")

	case r3.OperatorWeekdayIn, r3.OperatorTimeOfDayBetween:
		// Weekday/minute-of-day extraction is flavor-specific (PG EXTRACT(DOW),
		// MySQL DAYOFWEEK, SQLite strftime) and this dialect is flavor-neutral,
		// so there is no correct SQL to emit yet. Fail loudly rather than drop
		// the predicate and silently return unfiltered rows. Lowering lands with
		// the SQL flavor hook (see docs/plan-when-filters.md). Mongo and the file
		// engine execute these operators today.
		return "", fmt.Errorf("operator %s unsupported by the SQL dialect yet (docs/plan-when-filters.md)", &op)

	case r3.OperatorUnspecified:
		return "", fmt.Errorf("unsupported filter operator: %s", &op)
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
	// A relationship ("has") filter has no direct SQL form: the driver must
	// resolve it into a key-set In filter before translation. Reaching here
	// means the driver didn't lower it — fail loudly rather than emit bad SQL.
	if cf.Relation != "" {
		return SQLClause{}, fmt.Errorf(
			"relationship filter on %q must be lowered by the driver before SQL translation", cf.Relation,
		)
	}

	// Simple filter: Field is set.
	if cf.Field != nil {
		safeCol, err := SafeColumnExpr(cf.Field)
		if err != nil {
			return SQLClause{}, fmt.Errorf("unsafe filter field: %w", err)
		}

		joins, err := extractJoinFromField(cf.Field.String())
		if err != nil {
			return SQLClause{}, fmt.Errorf("unsafe join in filter field: %w", err)
		}

		// A nil value means IS NULL (Eq) or IS NOT NULL (Ne).
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

		if cf.Operator == r3.OperatorExists {
			clause := fmt.Sprintf("%s %s", safeCol, sqlIsNotNull)
			return SQLClause{Clause: clause, Args: nil, Joins: joins}, nil
		}

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

	// Compound filter (logical group): with Field empty, And or Or is non-empty.
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

	for _, child := range children {
		childClause, err := FilterToSQL(child)
		if err != nil {
			return SQLClause{}, fmt.Errorf("failed to translate child filter: %w", err)
		}
		conditions = append(conditions, childClause.Clause)
		args = append(args, childClause.Args...)
		joins = quick.Append(joins, childClause.Joins...)
	}

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

	safeCol, err := SafeColumnExpr(s.Column)
	if err != nil {
		return "", fmt.Errorf("unsafe sort column: %w", err)
	}

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

	// Offset = (PageNum - 1) * PageSize, so PageNum = (Offset / PageSize) + 1.
	pageSize := p.Limit
	pageNum := 1
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

	// No spaces means a bare field name (default direction).
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

	direction := parseSQLSortDirection(parts[1])

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
		return sqlDesc // default to descending
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
		return ""
	default:
		return ""
	}
}

// isBetweenOperator returns true if the operator is any of the between variants.
func isBetweenOperator(op r3.FilterOperatorSpec) bool {
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

// betweenToSQL converts a between filter (value is a [low, high] slice) to a
// compound condition, per variant:
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

// inToSQL converts an IN / NOT IN filter into "col IN (?, ?, ?)" with one
// placeholder per value, flattened into Args so raw database/sql and the ORM
// drivers alike bind them positionally.
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
		// A constant predicate references no column, so drop the join.
		return SQLClause{Clause: predicate, Args: nil, Joins: nil}
	}

	placeholders := strings.Repeat("?, ", len(args)-1) + "?"
	clause := fmt.Sprintf("%s %s (%s)", safeCol, op, placeholders)
	return SQLClause{Clause: clause, Args: args, Joins: joins}
}

// flattenInValues expands an IN value into individual arguments: a slice/array
// yields one per element, any other value a single-element set. A []byte is one
// scalar (a BLOB), not a list of bytes, matching how SQL drivers handle it.
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
