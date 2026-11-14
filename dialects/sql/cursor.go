package r3sql

import (
	"fmt"
	"strings"

	"github.com/amberpixels/r3"
)

// CursorToSQLClause builds a keyset pagination WHERE clause from decoded cursor
// values and the sort spec. A single column (created_at DESC) gives:
//
//	WHERE "created_at" < ?
//
// Multiple columns (created_at DESC, id ASC) give a compound condition:
//
//	WHERE ("created_at" < ?) OR ("created_at" = ? AND "id" > ?)
//
// direction picks > (forward) or < (backward), flipped per column sort direction.
func CursorToSQLClause(values r3.CursorValues, sorts r3.Sorts, direction r3.CursorDirection) (SQLClause, error) {
	return CursorToSQLClauseWith(values, sorts, direction, RenderOptions{})
}

// CursorToSQLClauseWith is [CursorToSQLClause] rendering JSON sort fields
// through opts.
func CursorToSQLClauseWith(
	values r3.CursorValues, sorts r3.Sorts, direction r3.CursorDirection, opts RenderOptions,
) (SQLClause, error) {
	if len(sorts) == 0 {
		return SQLClause{}, r3.ErrCursorRequiresSort
	}

	if len(values) == 0 {
		return SQLClause{}, nil
	}

	// Every sort column must carry a cursor value.
	for _, s := range sorts {
		col := s.Column.String()
		if _, ok := values[col]; !ok {
			return SQLClause{}, fmt.Errorf("%w: missing value for sort column %q", r3.ErrInvalidCursor, col)
		}
	}

	if len(sorts) == 1 {
		return singleColumnCursor(sorts[0], values, direction, opts)
	}
	return multiColumnCursor(sorts, values, direction, opts)
}

// cursorComparisonOp returns the keyset comparison operator: forward+ASC uses >,
// forward+DESC uses <, and backward flips it.
func cursorComparisonOp(sortDir r3.SortDirection, cursorDir r3.CursorDirection) SQLClauseOperator {
	// Unspecified defaults to DESC, matching SortToSQL.
	isAsc := sortDir == r3.SortDirectionAsc

	if cursorDir == r3.CursorBackward {
		isAsc = !isAsc
	}

	if isAsc {
		return SQLClauseOperatorGt
	}
	return SQLClauseOperatorLt
}

func singleColumnCursor(
	sort *r3.SortSpec, values r3.CursorValues, direction r3.CursorDirection, opts RenderOptions,
) (SQLClause, error) {
	col := sort.Column.String()
	safeCol, err := SafeFieldExpr(sort.Column, opts)
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

func multiColumnCursor(
	sorts r3.Sorts, values r3.CursorValues, direction r3.CursorDirection, opts RenderOptions,
) (SQLClause, error) {
	// Compound OR for multi-column keyset. For sorts [a DESC, b ASC, c DESC]:
	//   (a < ?)
	//   OR (a = ? AND b > ?)
	//   OR (a = ? AND b = ? AND c < ?)
	var orParts []string
	var allArgs []any

	for i := range sorts {
		var andParts []string
		var args []any

		// All preceding columns must be equal.
		for j := range i {
			safeCol, err := SafeFieldExpr(sorts[j].Column, opts)
			if err != nil {
				return SQLClause{}, fmt.Errorf("unsafe cursor column: %w", err)
			}
			andParts = append(andParts, safeCol+" = ?")
			args = append(args, values[sorts[j].Column.String()])
		}

		// Current column uses the comparison operator.
		safeCol, err := SafeFieldExpr(sorts[i].Column, opts)
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
