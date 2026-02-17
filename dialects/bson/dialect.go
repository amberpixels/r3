package r3bson

import (
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// OperatorToBSON converts a FilterOperatorSpec to a BSONOperator.
func OperatorToBSON(op r3.FilterOperatorSpec) (BSONOperator, error) {
	switch op {
	case r3.OperatorEq:
		return BSONOperatorEq, nil
	case r3.OperatorNe:
		return BSONOperatorNe, nil
	case r3.OperatorGt:
		return BSONOperatorGt, nil
	case r3.OperatorGte:
		return BSONOperatorGte, nil
	case r3.OperatorLt:
		return BSONOperatorLt, nil
	case r3.OperatorLte:
		return BSONOperatorLte, nil
	case r3.OperatorIn:
		return BSONOperatorIn, nil
	case r3.OperatorNotIn:
		return BSONOperatorNin, nil
	case r3.OperatorLike:
		return BSONOperatorRegex, nil
	case r3.OperatorILike:
		return BSONOperatorRegex, nil
	case r3.OperatorExists:
		return BSONOperatorExists, nil

	case r3.OperatorNotLike:
		// Handled specially via $not + $regex
		return BSONOperatorNot, nil

	case r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx:
		return "", fmt.Errorf("not implemented: %s", &op)

	case r3.OperatorUnspecified:
		fallthrough
	default:
		return "", fmt.Errorf("unsupported filter operator: %s", &op)
	}
}

// FilterToBSON converts a FilterSpec to a bson.D filter document.
func FilterToBSON(f *r3.FilterSpec) (bson.D, error) {
	// Case 1: Simple filter (Field is set).
	if f.Field != nil {
		fieldName := f.Field.String()

		if err := r3.ValidateIdentifier(fieldName); err != nil {
			return nil, fmt.Errorf("unsafe filter field: %w", err)
		}

		// Nil value: $eq null / $ne null
		if f.Value == nil {
			//nolint:exhaustive // handled by default case
			switch f.Operator {
			case r3.OperatorEq:
				return bson.D{{Key: fieldName, Value: bson.D{{Key: string(BSONOperatorEq), Value: nil}}}}, nil
			case r3.OperatorNe:
				return bson.D{{Key: fieldName, Value: bson.D{{Key: string(BSONOperatorNe), Value: nil}}}}, nil
			default:
				return nil, fmt.Errorf("unsupported operator %q for nil value", f.Operator)
			}
		}

		// LIKE: convert SQL-style wildcards to regex
		if f.Operator == r3.OperatorLike {
			pattern := likeToRegex(fmt.Sprintf("%v", f.Value))
			return bson.D{{Key: fieldName, Value: bson.D{
				{Key: string(BSONOperatorRegex), Value: pattern},
			}}}, nil
		}

		// ILIKE: same as LIKE but case-insensitive
		if f.Operator == r3.OperatorILike {
			pattern := likeToRegex(fmt.Sprintf("%v", f.Value))
			return bson.D{{Key: fieldName, Value: bson.D{
				{Key: string(BSONOperatorRegex), Value: pattern},
				{Key: string(BSONOperatorOptions), Value: "i"},
			}}}, nil
		}

		// NOT LIKE: $not with $regex
		if f.Operator == r3.OperatorNotLike {
			pattern := likeToRegex(fmt.Sprintf("%v", f.Value))
			return bson.D{{Key: fieldName, Value: bson.D{
				{Key: string(BSONOperatorNot), Value: bson.D{
					{Key: string(BSONOperatorRegex), Value: pattern},
				}},
			}}}, nil
		}

		// Standard operators
		bsonOp, err := OperatorToBSON(f.Operator)
		if err != nil {
			return nil, err
		}

		return bson.D{{Key: fieldName, Value: bson.D{{Key: string(bsonOp), Value: f.Value}}}}, nil
	}

	// Case 2: Compound filter (AND/OR group).
	if len(f.Or) > 0 {
		children := make(bson.A, 0, len(f.Or))
		for _, child := range f.Or {
			childDoc, err := FilterToBSON(child)
			if err != nil {
				return nil, fmt.Errorf("failed to translate child filter: %w", err)
			}
			children = append(children, childDoc)
		}
		return bson.D{{Key: string(BSONOperatorOr), Value: children}}, nil
	}

	if len(f.And) > 0 {
		children := make(bson.A, 0, len(f.And))
		for _, child := range f.And {
			childDoc, err := FilterToBSON(child)
			if err != nil {
				return nil, fmt.Errorf("failed to translate child filter: %w", err)
			}
			children = append(children, childDoc)
		}
		return bson.D{{Key: string(BSONOperatorAnd), Value: children}}, nil
	}

	return bson.D{}, nil
}

// FiltersToBSON converts a list of r3.Filters into a combined bson.D filter.
// Multiple filters are combined with $and.
func FiltersToBSON(filters r3.Filters) (bson.D, error) {
	if len(filters) == 0 {
		return bson.D{}, nil
	}

	if len(filters) == 1 {
		doc, err := FilterToBSON(filters[0])
		if err != nil {
			return nil, newError(err)
		}
		return doc, nil
	}

	// Multiple filters: wrap in $and
	children := make(bson.A, 0, len(filters))
	for _, f := range filters {
		doc, err := FilterToBSON(f)
		if err != nil {
			return nil, newError(err)
		}
		children = append(children, doc)
	}
	return bson.D{{Key: string(BSONOperatorAnd), Value: children}}, nil
}

// SortToBSON converts a SortSpec to a bson.E element (field: 1 or field: -1).
func SortToBSON(s *r3.SortSpec) (bson.E, error) {
	if s == nil {
		return bson.E{}, errors.New("sort spec cannot be nil")
	}

	fieldName := s.Column.String()
	if err := r3.ValidateIdentifier(fieldName); err != nil {
		return bson.E{}, fmt.Errorf("unsafe sort field: %w", err)
	}

	direction := -1 // default to descending (matches SQL dialect behavior)
	if s.Direction == r3.SortDirectionAsc {
		direction = 1
	}

	// Note: MongoDB does not natively support NULLS FIRST/LAST.
	// This would require aggregation pipeline with $ifNull.
	// For now, we skip NullsPosition (same limitation as some SQL drivers).

	return bson.E{Key: fieldName, Value: direction}, nil
}

// SortsToBSON converts a slice of SortSpec to a bson.D sort document.
func SortsToBSON(sorts r3.Sorts) (bson.D, error) {
	if len(sorts) == 0 {
		return bson.D{}, nil
	}

	result := make(bson.D, 0, len(sorts))
	for _, s := range sorts {
		elem, err := SortToBSON(s)
		if err != nil {
			return nil, err
		}
		result = append(result, elem)
	}
	return result, nil
}

// FieldToBSON converts a FieldSpec to a BSON field name string.
func FieldToBSON(f *r3.FieldSpec) string {
	return f.String()
}

// FieldsToBSON converts r3.Fields to a bson.D projection document.
// Each field is set to 1 (include). The _id field is always included.
func FieldsToBSON(fields r3.Fields) bson.D {
	if len(fields) == 0 {
		return nil
	}

	projection := make(bson.D, 0, len(fields)+1)
	hasID := false
	for _, f := range fields {
		if f == nil {
			continue
		}
		name := f.String()
		if name == "_id" {
			hasID = true
		}
		projection = append(projection, bson.E{Key: name, Value: 1})
	}
	if !hasID {
		// Always include _id for identity
		projection = append(bson.D{{Key: "_id", Value: 1}}, projection...)
	}
	return projection
}
