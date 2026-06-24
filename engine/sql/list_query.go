package enginesql

import (
	"fmt"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

// PreparedListQuery holds pre-computed SQL components derived from an r3.Query.
// It is the result of converting r3 filters, sorts, and pagination into SQL-ready pieces
// that any driver (GORM, Bun, go-pg, database/sql) can consume.
//
// This eliminates the duplicated r3-to-SQL conversion logic across drivers.
type PreparedListQuery struct {
	// Clauses is the list of SQL WHERE clauses with their args and joins.
	Clauses r3sql.SQLClauses

	// Sorts is the list of SQL ORDER BY expressions.
	Sorts []r3sql.SQLSort

	// IsPaginated indicates whether offset-based pagination is active.
	IsPaginated bool

	// Limit and Offset are set when IsPaginated is true.
	Limit  int
	Offset int

	// IsCursorPaginated indicates whether cursor/keyset pagination is active.
	// When true, CursorClause contains the keyset WHERE condition and CursorLimit
	// contains the LIMIT. No OFFSET or COUNT query should be used.
	IsCursorPaginated bool

	// CursorClause is the keyset WHERE clause (may be empty for the first page).
	CursorClause r3sql.SQLClause

	// CursorLimit is the maximum number of results for cursor pagination.
	CursorLimit int

	// CursorBackward indicates a "before" cursor. Backward keyset pagination
	// must scan in the reversed sort order (so LIMIT takes the rows immediately
	// preceding the cursor) and then reverse the result slice back to the
	// requested order. See OrderBySorts.
	CursorBackward bool

	// Query is the merged r3.Query (defaults + user args) for access to
	// Preloads, IncludeTrashed, and other fields that are driver-specific.
	Query r3.Query
}

// PrepareListQuery merges defaults with user query args, then converts filters,
// sorts, and pagination into SQL-ready components.
//
// Drivers call this once at the start of List() and then consume the result
// using their ORM-specific APIs.
func PrepareListQuery(dm *DefaultsManager, qarg ...r3.Query) (PreparedListQuery, error) {
	return PrepareMergedListQuery(dm.MergeListQuery(qarg...))
}

// PrepareMergedListQuery builds the SQL components for an already-merged query
// (defaults + user args). It is the half of PrepareListQuery after the merge,
// exposed so a driver can transform the merged query — e.g. lower relationship
// ("has") filters into key-set In filters — before the clauses are built.
func PrepareMergedListQuery(q r3.Query) (PreparedListQuery, error) {
	var p PreparedListQuery
	p.Query = q

	// Convert filters to SQL clauses
	clauses, err := r3sql.FiltersToSQL(q.Filters)
	if err != nil {
		return p, fmt.Errorf("failed to convert filters to SQL: %w", err)
	}
	p.Clauses = clauses

	// Convert sorts to SQL
	if len(q.Sorts) > 0 {
		sorts, err := r3sql.SortsToSQL(q.Sorts)
		if err != nil {
			return p, fmt.Errorf("failed to convert sorts to SQL: %w", err)
		}
		p.Sorts = sorts
	}

	// Compute pagination: cursor takes precedence over offset-based
	if q.Cursor != nil {
		if len(q.Sorts) == 0 {
			return p, r3.ErrCursorRequiresSort
		}

		p.IsCursorPaginated = true
		p.CursorLimit = q.Cursor.GetLimit()
		p.CursorBackward = q.Cursor.Direction() == r3.CursorBackward

		token := q.Cursor.Token()
		if token != "" {
			values, err := r3.DecodeCursor(token)
			if err != nil {
				return p, fmt.Errorf("failed to decode cursor: %w", err)
			}
			cursorClause, err := r3sql.CursorToSQLClause(values, q.Sorts, q.Cursor.Direction())
			if err != nil {
				return p, fmt.Errorf("failed to build cursor clause: %w", err)
			}
			p.CursorClause = cursorClause
		}
	} else if q.Pagination != nil && q.Pagination.IsPaginated() {
		p.IsPaginated = true
		p.Limit, p.Offset = q.Pagination.ToLimitOffset()
	}

	return p, nil
}

// OrderBySorts returns the ORDER BY expressions to apply to the main SELECT.
//
// For forward queries it is just Sorts. For a backward ("before") cursor it
// returns the sorts with reversed direction: the rows immediately preceding the
// cursor must be scanned in the opposite order under LIMIT, then reversed back
// to the requested order by the caller (see CursorBackward).
func (p *PreparedListQuery) OrderBySorts() ([]r3sql.SQLSort, error) {
	if !p.CursorBackward {
		return p.Sorts, nil
	}
	return r3sql.SortsToSQL(reverseSortDirections(p.Query.Sorts))
}

// reverseSortDirections returns a copy of sorts with each direction (and NULLS
// position) flipped. Unspecified direction defaults to DESC (matching
// SortToSQL), so its reverse is ASC.
func reverseSortDirections(sorts r3.Sorts) r3.Sorts {
	reversed := make(r3.Sorts, len(sorts))
	for i, s := range sorts {
		c := s.Clone()
		switch c.Direction {
		case r3.SortDirectionAsc:
			c.Direction = r3.SortDirectionDesc
		case r3.SortDirectionDesc, r3.SortDirectionUnspecified:
			c.Direction = r3.SortDirectionAsc
		}
		switch c.NullsPosition {
		case r3.NullsPositionFirst:
			c.NullsPosition = r3.NullsPositionLast
		case r3.NullsPositionLast:
			c.NullsPosition = r3.NullsPositionFirst
		case r3.NullsPositionNotSpecified:
			// leave unspecified
		}
		reversed[i] = c
	}
	return reversed
}

// Joins returns the deduplicated list of SQL joins from the clauses.
func (p *PreparedListQuery) Joins() []r3sql.SQLColumn {
	if len(p.Clauses) == 0 {
		return nil
	}
	return p.Clauses.Joins()
}

// FinalizeCount returns (entities, totalCount) with the correct total.
// If pagination was not active, totalCount is simply len(entities).
//
// Deprecated: Use r3.FinalizeCount directly.
func FinalizeCount[T any](entities []T, paginatedCount int64, isPaginated bool) ([]T, int64) {
	return r3.FinalizeCount(entities, paginatedCount, isPaginated)
}
