package enginesql

import (
	"fmt"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

// PreparedListQuery holds the SQL components derived from an r3.Query - filters,
// sorts, and pagination converted into SQL-ready pieces that any driver (GORM,
// Bun, go-pg, database/sql) can consume, so the r3-to-SQL translation lives once.
type PreparedListQuery struct {
	// Clauses is the SQL WHERE clauses with their args and joins.
	Clauses r3sql.SQLClauses

	// Sorts is the SQL ORDER BY expressions.
	Sorts []r3sql.SQLSort

	// IsPaginated reports offset-based pagination; Limit/Offset are then set.
	IsPaginated bool
	Limit       int
	Offset      int

	// IsCursorPaginated reports cursor/keyset pagination. When true, use
	// CursorClause/CursorLimit and no OFFSET or COUNT query.
	IsCursorPaginated bool

	// CursorClause is the keyset WHERE clause (empty for the first page).
	CursorClause r3sql.SQLClause

	// CursorLimit is the row limit for cursor pagination.
	CursorLimit int

	// CursorBackward marks a "before" cursor. Backward keyset pagination scans in
	// reversed sort order (so LIMIT takes the rows immediately preceding the
	// cursor), then the caller reverses the slice back. See OrderBySorts.
	CursorBackward bool

	// Query is the merged r3.Query (defaults + args), for Preloads, IncludeTrashed,
	// and other driver-specific fields.
	Query r3.Query
}

// PrepareListQuery merges defaults with args, then converts filters, sorts, and
// pagination into SQL components. Drivers call it once at the start of List().
func PrepareListQuery(dm *DefaultsManager, qarg ...r3.Query) (PreparedListQuery, error) {
	return PrepareMergedListQuery(dm.MergeListQuery(qarg...))
}

// PrepareMergedListQuery builds the SQL components for an already-merged query -
// the half of PrepareListQuery after the merge, exposed so a driver can transform
// the query first (e.g. lower relationship "has" filters into key-set In filters).
//
// It applies no value codecs; a driver whose schema declares codecs must call
// [PrepareMergedListQuerySchema] to encode codec'd filter/cursor args to stored
// form.
//
//nolint:lostfield // thin wrapper: delegates to PrepareMergedListQuerySchema, which reads the query and populates every PreparedListQuery field
func PrepareMergedListQuery(q r3.Query) (PreparedListQuery, error) {
	return PrepareMergedListQuerySchema(r3.Schema{}, q)
}

// PrepareMergedListQuerySchema is [PrepareMergedListQuery] with schema-driven
// value codecs applied: filter args and decoded cursor keys on codec'd attributes
// are encoded to stored form so predicates compare against stored column values.
// A zero schema (no codecs) behaves exactly like PrepareMergedListQuery.
//
//nolint:lostfield // every PreparedListQuery field is assigned imperatively below; some branches are conditional by design (zero value means "not requested")
func PrepareMergedListQuerySchema(schema r3.Schema, q r3.Query) (PreparedListQuery, error) {
	var p PreparedListQuery

	// Encode codec'd filter args to stored form before conversion.
	filters, err := r3.EncodeFilterCodecs(schema, q.Filters)
	if err != nil {
		return p, fmt.Errorf("failed to encode filter codecs: %w", err)
	}
	q.Filters = filters
	p.Query = q

	clauses, err := r3sql.FiltersToSQL(q.Filters)
	if err != nil {
		return p, fmt.Errorf("failed to convert filters to SQL: %w", err)
	}
	p.Clauses = clauses

	if len(q.Sorts) > 0 {
		sorts, err := r3sql.SortsToSQL(q.Sorts)
		if err != nil {
			return p, fmt.Errorf("failed to convert sorts to SQL: %w", err)
		}
		p.Sorts = sorts
	}

	// Cursor pagination takes precedence over offset.
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
			// Encode codec'd cursor keys to stored form for the keyset predicate.
			values, err = r3.EncodeCursorCodecs(schema, values)
			if err != nil {
				return p, fmt.Errorf("failed to encode cursor codecs: %w", err)
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

// OrderBySorts returns the main SELECT's ORDER BY. Forward queries use Sorts; a
// backward ("before") cursor reverses each direction, so the rows preceding the
// cursor are scanned under LIMIT and reversed back by the caller (see
// CursorBackward).
func (p *PreparedListQuery) OrderBySorts() ([]r3sql.SQLSort, error) {
	if !p.CursorBackward {
		return p.Sorts, nil
	}
	return r3sql.SortsToSQL(reverseSortDirections(p.Query.Sorts))
}

// reverseSortDirections copies sorts with each direction and NULLS position
// flipped. Unspecified direction defaults to DESC (matching SortToSQL), so its
// reverse is ASC.
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

// FinalizeCount returns (entities, totalCount), using len(entities) when
// pagination was inactive.
//
// Deprecated: Use r3.FinalizeCount directly.
func FinalizeCount[T any](entities []T, paginatedCount int64, isPaginated bool) ([]T, int64) {
	return r3.FinalizeCount(entities, paginatedCount, isPaginated)
}
