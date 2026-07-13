package enginemongo

import (
	"fmt"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// PreparedListQuery holds the BSON-ready pieces (filter, sort, projection,
// pagination) derived from an r3.Query.
type PreparedListQuery struct {
	Filter     bson.D // combined BSON filter
	Sort       bson.D
	Projection bson.D // field selection

	// IsPaginated marks offset pagination; Limit and Offset (skip) are then set.
	IsPaginated bool
	Limit       int64
	Offset      int64

	// IsCursorPaginated marks cursor/keyset pagination: use CursorFilter and
	// CursorLimit, with no skip and no count query.
	IsCursorPaginated bool
	CursorFilter      bson.D // keyset filter; empty for the first page
	CursorLimit       int64

	// Query is the merged r3.Query, for Preloads, IncludeTrashed, etc.
	Query r3.Query
}

// PrepareListQuery merges defaults with user args, then converts filters, sorts,
// and pagination into BSON via [PrepareMergedListQuery].
func PrepareListQuery(dm *r3.DefaultsManager, schema r3.Schema, qarg ...r3.Query) (PreparedListQuery, error) {
	return PrepareMergedListQuery(schema, dm.MergeListQuery(qarg...))
}

// PrepareMergedListQuery converts an already-merged query's filters, sorts, and
// pagination into BSON. Cursor pagination takes precedence over offset. It is the
// half of [PrepareListQuery] after the merge, exposed so a driver can transform the
// query first (e.g. lower relationship "has" filters into key-set In filters).
//
// schema drives value-codec encoding of filter and cursor arguments to stored form
// (e.g. a time.Time bound against an int column); a zero schema (no codecs) leaves
// the arguments untouched.
func PrepareMergedListQuery(schema r3.Schema, q r3.Query) (PreparedListQuery, error) {
	var p PreparedListQuery

	// Encode codec'd filter args to stored form before conversion.
	filters, err := r3.EncodeFilterCodecs(schema, q.Filters)
	if err != nil {
		return p, fmt.Errorf("failed to encode filter codecs: %w", err)
	}
	q.Filters = filters
	p.Query = q

	filter, err := r3bson.FiltersToBSON(q.Filters)
	if err != nil {
		return p, fmt.Errorf("failed to convert filters to BSON: %w", err)
	}
	p.Filter = filter

	if len(q.Sorts) > 0 {
		sort, err := r3bson.SortsToBSON(q.Sorts)
		if err != nil {
			return p, fmt.Errorf("failed to convert sorts to BSON: %w", err)
		}
		p.Sort = sort
	}

	if len(q.Fields) > 0 {
		p.Projection = r3bson.FieldsToBSON(q.Fields)
	}

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
			// Encode codec'd cursor keys to stored form for the keyset predicate.
			values, err = r3.EncodeCursorCodecs(schema, values)
			if err != nil {
				return p, fmt.Errorf("failed to encode cursor codecs: %w", err)
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
