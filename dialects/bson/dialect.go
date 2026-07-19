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
		// $not + $regex, applied by the caller.
		return BSONOperatorNot, nil

	case r3.OperatorBetween,
		r3.OperatorBetweenEx,
		r3.OperatorBetweenExInc,
		r3.OperatorBetweenIncEx:
		// Between is a compound condition, lowered directly in FilterToBSON.
		return "", errors.New("between operators must be handled via FilterToBSON, not OperatorToBSON")

	case r3.OperatorWeekdayIn, r3.OperatorTimeOfDayBetween:
		// Time-component operators lower to $expr, handled in FilterToBSON.
		return "", errors.New("time-component operators must be handled via FilterToBSON, not OperatorToBSON")

	case r3.OperatorUnspecified:
		fallthrough
	default:
		return "", fmt.Errorf("unsupported filter operator: %s", &op)
	}
}

// FilterToBSON converts a FilterSpec to a bson.D filter document.
func FilterToBSON(f *r3.FilterSpec) (bson.D, error) {
	// A relationship ("has") filter has no direct BSON form: Mongo has no
	// server-side join, so the driver must lower it to a key-set In/NotIn via a
	// pre-query before translation. Reaching here unresolved means it would
	// otherwise be silently dropped (returning unfiltered rows), so fail loudly.
	if f.Relation != "" {
		return nil, fmt.Errorf(
			"relationship filter on %q must be resolved by the driver before BSON translation", f.Relation)
	}

	// Simple filter (Field set).
	if f.Field != nil {
		fieldName := f.Field.String()

		if err := r3.ValidateIdentifier(fieldName); err != nil {
			return nil, fmt.Errorf("unsafe filter field: %w", err)
		}

		// Nil value: $eq null / $ne null.
		if f.Value == nil {
			//nolint:exhaustive // handled by default case
			switch f.Operator {
			case r3.OperatorEq:
				return bson.D{{Key: fieldName, Value: bson.D{{Key: string(BSONOperatorEq), Value: nil}}}}, nil
			case r3.OperatorNe:
				return bson.D{{Key: fieldName, Value: bson.D{{Key: string(BSONOperatorNe), Value: nil}}}}, nil
			default:
				return nil, fmt.Errorf("unsupported operator %v for nil value", f.Operator)
			}
		}

		// LIKE: SQL wildcards to regex.
		if f.Operator == r3.OperatorLike {
			pattern := likeToRegex(fmt.Sprintf("%v", f.Value))
			return bson.D{{Key: fieldName, Value: bson.D{
				{Key: string(BSONOperatorRegex), Value: pattern},
			}}}, nil
		}

		// ILIKE: LIKE plus the case-insensitive option.
		if f.Operator == r3.OperatorILike {
			pattern := likeToRegex(fmt.Sprintf("%v", f.Value))
			return bson.D{{Key: fieldName, Value: bson.D{
				{Key: string(BSONOperatorRegex), Value: pattern},
				{Key: string(BSONOperatorOptions), Value: "i"},
			}}}, nil
		}

		// NOT LIKE: $not with $regex.
		if f.Operator == r3.OperatorNotLike {
			pattern := likeToRegex(fmt.Sprintf("%v", f.Value))
			return bson.D{{Key: fieldName, Value: bson.D{
				{Key: string(BSONOperatorNot), Value: bson.D{
					{Key: string(BSONOperatorRegex), Value: pattern},
				}},
			}}}, nil
		}

		if isBetweenOperator(f.Operator) {
			return betweenToBSON(fieldName, f.Operator, f.Value)
		}

		if f.Operator == r3.OperatorWeekdayIn || f.Operator == r3.OperatorTimeOfDayBetween {
			return timePatternToBSON(fieldName, f.Operator, f.Value)
		}

		bsonOp, err := OperatorToBSON(f.Operator)
		if err != nil {
			return nil, err
		}

		return bson.D{{Key: fieldName, Value: bson.D{{Key: string(bsonOp), Value: f.Value}}}}, nil
	}

	// Compound filter (AND/OR group).
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

// FiltersToBSON converts r3.Filters into a combined bson.D, joining multiple
// filters with $and.
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

	// MongoDB has no native NULLS FIRST/LAST (would need an $ifNull pipeline), so
	// NullsPosition is dropped - the same degradation as some SQL drivers.

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

// FieldsToBSON converts r3.Fields to a bson.D inclusion projection (each field
// set to 1). _id is always included, even when not listed.
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
		projection = append(bson.D{{Key: "_id", Value: 1}}, projection...)
	}
	return projection
}

// isBetweenOperator reports whether op is any between variant.
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

// betweenToBSON lowers a between filter (value is a 2-element [low, high] slice)
// to a compound condition, picking $gt/$gte and $lt/$lte per variant's
// inclusivity.
func betweenToBSON(fieldName string, op r3.FilterOperatorSpec, value any) (bson.D, error) {
	low, high, err := r3.ExtractBetweenBounds(value)
	if err != nil {
		return nil, err
	}

	var lowOp, highOp BSONOperator
	//nolint:exhaustive // only between variants reach here, guarded by isBetweenOperator
	switch op {
	case r3.OperatorBetween:
		lowOp, highOp = BSONOperatorGte, BSONOperatorLte
	case r3.OperatorBetweenEx:
		lowOp, highOp = BSONOperatorGt, BSONOperatorLt
	case r3.OperatorBetweenExInc:
		lowOp, highOp = BSONOperatorGt, BSONOperatorLte
	case r3.OperatorBetweenIncEx:
		lowOp, highOp = BSONOperatorGte, BSONOperatorLt
	default:
		return nil, fmt.Errorf("unexpected between operator: %v", op)
	}

	return bson.D{{Key: fieldName, Value: bson.D{
		{Key: string(lowOp), Value: low},
		{Key: string(highOp), Value: high},
	}}}, nil
}

// timePatternToBSON lowers a time-component operator (weekday_in, tod_between) to
// an $expr document. These compare a component extracted from the field's date
// (its weekday or minute-of-day), which has no plain-query form, so $expr with
// the aggregation date operators is the only expression. The stored BSON date is
// read as UTC (no timezone argument), matching the operators' wall-clock-as-is
// contract. $expr cannot use an index; see docs/plan-when-filters.md.
func timePatternToBSON(fieldName string, op r3.FilterOperatorSpec, value any) (bson.D, error) {
	fieldRef := "$" + fieldName

	//nolint:exhaustive // only the two time-component operators reach here (guarded by the caller)
	switch op {
	case r3.OperatorWeekdayIn:
		days, err := r3.WeekdaysValue(value)
		if err != nil {
			return nil, err
		}
		// $dayOfWeek numbers Sunday..Saturday as 1..7; Go weekdays are 0..6.
		mongoDays := make(bson.A, 0, len(days))
		for _, d := range days {
			mongoDays = append(mongoDays, int(d)+1)
		}
		dow := bson.D{{Key: string(BSONOperatorDayOfWeek), Value: fieldRef}}
		return exprDoc(bson.D{{Key: string(BSONOperatorIn), Value: bson.A{dow, mongoDays}}}), nil

	case r3.OperatorTimeOfDayBetween:
		lo, hi, err := r3.TimeOfDayBounds(value)
		if err != nil {
			return nil, err
		}
		// minuteOfDay = $hour*60 + $minute.
		minuteOfDay := bson.D{{Key: string(BSONOperatorAdd), Value: bson.A{
			bson.D{{Key: string(BSONOperatorMultiply), Value: bson.A{
				bson.D{{Key: string(BSONOperatorHour), Value: fieldRef}},
				60,
			}}},
			bson.D{{Key: string(BSONOperatorMinute), Value: fieldRef}},
		}}}
		lowerBound := bson.D{{Key: string(BSONOperatorGte), Value: bson.A{minuteOfDay, lo}}}
		upperBound := bson.D{{Key: string(BSONOperatorLt), Value: bson.A{minuteOfDay, hi}}}
		// lo <= hi is a single interval (AND); lo > hi wraps midnight (OR).
		joiner := BSONOperatorAnd
		if lo > hi {
			joiner = BSONOperatorOr
		}
		return exprDoc(bson.D{{Key: string(joiner), Value: bson.A{lowerBound, upperBound}}}), nil

	default:
		return nil, fmt.Errorf("timePatternToBSON: unexpected operator %v", op)
	}
}

// exprDoc wraps an aggregation expression in a top-level {$expr: ...} document.
func exprDoc(expr bson.D) bson.D {
	return bson.D{{Key: string(BSONOperatorExpr), Value: expr}}
}
