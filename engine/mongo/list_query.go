package enginemongo

import (
	"fmt"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// PreparedListQuery holds pre-computed BSON components derived from an r3.Query.
// It is the result of converting r3 filters, sorts, and pagination into BSON-ready
// pieces that the MongoDB driver can consume.
type PreparedListQuery struct {
	// Filter is the combined BSON filter document.
	Filter bson.D

	// Sort is the BSON sort document.
	Sort bson.D

	// Projection is the BSON projection document (field selection).
	Projection bson.D

	// IsPaginated indicates whether offset-based pagination is active.
	IsPaginated bool

	// Limit and Offset (skip) are set when IsPaginated is true.
	Limit  int64
	Offset int64

	// IsCursorPaginated indicates whether cursor/keyset pagination is active.
	// When true, CursorFilter contains the keyset filter and CursorLimit
	// contains the LIMIT. No skip or count query should be used.
	IsCursorPaginated bool

	// CursorFilter is the keyset filter (may be empty for the first page).
	CursorFilter bson.D

	// CursorLimit is the maximum number of results for cursor pagination.
	CursorLimit int64

	// Query is the merged r3.Query (defaults + user args) for access to
	// Preloads, IncludeTrashed, and other fields.
	Query r3.Query
}

// PrepareListQuery merges defaults with user query args, then converts filters,
// sorts, and pagination into BSON-ready components.
func PrepareListQuery(dm *r3.DefaultsManager, qarg ...r3.Query) (PreparedListQuery, error) {
	q := dm.MergeListQuery(qarg...)

	var p PreparedListQuery
	p.Query = q

	// Convert filters to BSON
	filter, err := r3bson.FiltersToBSON(q.Filters)
	if err != nil {
		return p, fmt.Errorf("failed to convert filters to BSON: %w", err)
	}
	p.Filter = filter

	// Convert sorts to BSON
	if len(q.Sorts) > 0 {
		sort, err := r3bson.SortsToBSON(q.Sorts)
		if err != nil {
			return p, fmt.Errorf("failed to convert sorts to BSON: %w", err)
		}
		p.Sort = sort
	}

	// Convert field selection to BSON projection
	if len(q.Fields) > 0 {
		p.Projection = r3bson.FieldsToBSON(q.Fields)
	}

	// Compute pagination: cursor takes precedence over offset-based
	if q.Cursor != nil {
		if len(q.Sorts) == 0 {
			return p, r3.ErrCursorRequiresSort
		}

		p.IsCursorPaginated = true
		p.CursorLimit = int64(q.Cursor.GetLimit())

		token := q.Cursor.Token()
		if token != "" {
			values, err := r3.DecodeCursor(token)
			if err != nil {
				return p, fmt.Errorf("failed to decode cursor: %w", err)
			}
			cursorFilter, err := r3bson.CursorToBSON(values, q.Sorts, q.Cursor.Direction())
			if err != nil {
				return p, fmt.Errorf("failed to build cursor filter: %w", err)
			}
			p.CursorFilter = cursorFilter
		}
	} else if q.Pagination != nil && q.Pagination.IsPaginated() {
		p.IsPaginated = true
		limit, offset := q.Pagination.ToLimitOffset()
		p.Limit = int64(limit)
		p.Offset = int64(offset)
	}

	return p, nil
}
