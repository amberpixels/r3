package r3bson

import (
	"fmt"

	"github.com/amberpixels/r3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// CursorToBSON builds a keyset pagination filter from decoded cursor values and
// the active sorts. A single column yields {col: {$lt: v}}; multiple columns
// yield the lexicographic $or/$and expansion, e.g.
//
//	{$or: [{created_at: {$lt: v1}}, {$and: [{created_at: {$eq: v1}}, {id: {$gt: v2}}]}]}
//
// $gt vs $lt is chosen per column from its sort direction, flipped for a backward
// cursor.
func CursorToBSON(values r3.CursorValues, sorts r3.Sorts, direction r3.CursorDirection) (bson.D, error) {
	if len(sorts) == 0 {
		return nil, r3.ErrCursorRequiresSort
	}

	if len(values) == 0 {
		return bson.D{}, nil
	}

	// Every sort column needs a cursor value.
	for _, s := range sorts {
		col := s.Column.String()
		if _, ok := values[col]; !ok {
			return nil, fmt.Errorf("%w: missing value for sort column %q", r3.ErrInvalidCursor, col)
		}
	}

	if len(sorts) == 1 {
		return singleColumnCursorBSON(sorts[0], values, direction)
	}
	return multiColumnCursorBSON(sorts, values, direction)
}

// cursorBSONOp returns the BSON comparison operator for cursor pagination.
func cursorBSONOp(sortDir r3.SortDirection, cursorDir r3.CursorDirection) BSONOperator {
	isAsc := sortDir == r3.SortDirectionAsc

	if cursorDir == r3.CursorBackward {
		isAsc = !isAsc
	}

	if isAsc {
		return BSONOperatorGt
	}
	return BSONOperatorLt
}

func singleColumnCursorBSON(sort *r3.SortSpec, values r3.CursorValues, direction r3.CursorDirection) (bson.D, error) {
	col := sort.Column.String()
	if err := r3.ValidateIdentifier(col); err != nil {
		return nil, fmt.Errorf("unsafe cursor field: %w", err)
	}

	op := cursorBSONOp(sort.Direction, direction)
	return bson.D{{Key: col, Value: bson.D{{Key: string(op), Value: values[col]}}}}, nil
}

func multiColumnCursorBSON(sorts r3.Sorts, values r3.CursorValues, direction r3.CursorDirection) (bson.D, error) {
	// Lexicographic keyset expansion. For sorts [a DESC, b ASC]:
	//   $or: [{a: {$lt: v1}}, {$and: [{a: {$eq: v1}}, {b: {$gt: v2}}]}]
	orConditions := make(bson.A, 0, len(sorts))

	for i := range sorts {
		col := sorts[i].Column.String()
		if err := r3.ValidateIdentifier(col); err != nil {
			return nil, fmt.Errorf("unsafe cursor field: %w", err)
		}

		if i == 0 {
			// First level: bare comparison.
			op := cursorBSONOp(sorts[i].Direction, direction)
			orConditions = append(orConditions, bson.D{
				{Key: col, Value: bson.D{{Key: string(op), Value: values[col]}}},
			})
			continue
		}

		// Deeper levels: preceding columns equal, this one compared.
		andConditions := make(bson.A, 0, i+1)
		for j := range i {
			prevCol := sorts[j].Column.String()
			if err := r3.ValidateIdentifier(prevCol); err != nil {
				return nil, fmt.Errorf("unsafe cursor field: %w", err)
			}
			andConditions = append(andConditions, bson.D{
				{Key: prevCol, Value: bson.D{{Key: string(BSONOperatorEq), Value: values[prevCol]}}},
			})
		}

		op := cursorBSONOp(sorts[i].Direction, direction)
		andConditions = append(andConditions, bson.D{
			{Key: col, Value: bson.D{{Key: string(op), Value: values[col]}}},
		})

		orConditions = append(orConditions, bson.D{
			{Key: string(BSONOperatorAnd), Value: andConditions},
		})
	}

	return bson.D{{Key: string(BSONOperatorOr), Value: orConditions}}, nil
}
