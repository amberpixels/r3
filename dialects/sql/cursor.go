package r3sql

import (
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

// CursorToSQLClause generates a keyset pagination WHERE clause from decoded cursor values
// and the current sort specification.
//
// For a single sort column (e.g., created_at DESC), it produces:
//
//	WHERE "created_at" < ?
//
// For multiple sort columns (e.g., created_at DESC, id ASC), it produces a compound condition:
//
//	WHERE ("created_at" < ?) OR ("created_at" = ? AND "id" > ?)
//
// The direction parameter controls whether comparison operators are > (forward) or < (backward),
// flipped based on the sort direction of each column.
func CursorToSQLClause(values r3.CursorValues, sorts r3.Sorts, direction r3.CursorDirection) (SQLClause, error) {
	if len(sorts) == 0 {
		return SQLClause{}, r3.ErrCursorRequiresSort
	}

	if len(values) == 0 {
		return SQLClause{}, nil
	}

	// Validate that all sort columns have cursor values
	for _, s := range sorts {
		col := s.Column.String()
		if _, ok := values[col]; !ok {
			return SQLClause{}, fmt.Errorf("%w: missing value for sort column %q", r3.ErrInvalidCursor, col)
		}
	}

	// Single column: simple comparison
	if len(sorts) == 1 {
		return singleColumnCursor(sorts[0], values, direction)
	}

	// Multi-column: compound OR condition
	return multiColumnCursor(sorts, values, direction)
}

// cursorComparisonOp returns the SQL comparison operator for cursor pagination.
// For forward direction + DESC sort: use "<" (get items with smaller values).
// For forward direction + ASC sort: use ">" (get items with larger values).
// Backward direction flips the operator.
func cursorComparisonOp(sortDir r3.SortDirection, cursorDir r3.CursorDirection) SQLClauseOperator {
	// Default unspecified to DESC (matching SortToSQL behavior)
	isAsc := sortDir == r3.SortDirectionAsc

	if cursorDir == r3.CursorBackward {
		isAsc = !isAsc
	}

	if isAsc {
		return SQLClauseOperatorGt
	}
	return SQLClauseOperatorLt
}

func singleColumnCursor(sort *r3.SortSpec, values r3.CursorValues, direction r3.CursorDirection) (SQLClause, error) {
	col := sort.Column.String()
	safeCol, err := SafeColumnExpr(sort.Column)
	if err != nil {
		return SQLClause{}, fmt.Errorf("unsafe cursor column: %w", err)
	}

	op := cursorComparisonOp(sort.Direction, direction)
	clause := fmt.Sprintf("%s %s ?", safeCol, op)

	return SQLClause{
		Clause: clause,
		Args:   []any{values[col]},
	}, nil
}

func multiColumnCursor(sorts r3.Sorts, values r3.CursorValues, direction r3.CursorDirection) (SQLClause, error) {
	// Build compound OR condition for multi-column keyset pagination.
	// For sorts [a DESC, b ASC, c DESC]:
	//   (a < ?)
	//   OR (a = ? AND b > ?)
	//   OR (a = ? AND b = ? AND c < ?)
	var orParts []string
	var allArgs []any

	for i := range sorts {
		var andParts []string
		var args []any

		// All preceding columns must be equal
		for j := range i {
			safeCol, err := SafeColumnExpr(sorts[j].Column)
			if err != nil {
				return SQLClause{}, fmt.Errorf("unsafe cursor column: %w", err)
			}
			andParts = append(andParts, fmt.Sprintf("%s = ?", safeCol))
			args = append(args, values[sorts[j].Column.String()])
		}

		// Current column uses comparison
		safeCol, err := SafeColumnExpr(sorts[i].Column)
		if err != nil {
			return SQLClause{}, fmt.Errorf("unsafe cursor column: %w", err)
		}
		op := cursorComparisonOp(sorts[i].Direction, direction)
		andParts = append(andParts, fmt.Sprintf("%s %s ?", safeCol, op))
		args = append(args, values[sorts[i].Column.String()])

		orParts = append(orParts, "("+strings.Join(andParts, " AND ")+")")
		allArgs = append(allArgs, args...)
	}

	combined := "(" + strings.Join(orParts, " OR ") + ")"
	return SQLClause{
		Clause: combined,
		Args:   allArgs,
	}, nil
}
