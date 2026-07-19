package r3

// FilterOperatorSpec is a filter operator. This is the full set r3 aims to
// support; not every dialect supports every one.
// TODO: might be refactored into a more complex struct
type FilterOperatorSpec int8

const (
	OperatorUnspecified      FilterOperatorSpec = iota
	OperatorEq                                  // =
	OperatorNe                                  // !=
	OperatorExists                              // exists
	OperatorGt                                  // >
	OperatorGte                                 // >=
	OperatorLt                                  // <
	OperatorLte                                 // <=
	OperatorBetween                             // between_inc meaning []
	OperatorBetweenEx                           // between_exc meaning ()
	OperatorBetweenExInc                        // between_exc_inc meaning (]
	OperatorBetweenIncEx                        // between_inc_exc meaning [)
	OperatorIn                                  // in
	OperatorNotIn                               // not in
	OperatorLike                                // like
	OperatorNotLike                             // not like
	OperatorILike                               // ilike (like + case insensitive)
	OperatorWeekdayIn                           // weekday_in: weekday-of(field) ∈ value (see r3_filter_timepattern.go)
	OperatorTimeOfDayBetween                    // tod_between: minute-of-day-of(field) ∈ [lo, hi)
)

// String returns a debug label for the operator.
// Note: Protect with the `exhausted` linter.
func (op *FilterOperatorSpec) String() string {
	if op == nil {
		return ""
	}

	switch *op {
	case OperatorEq:
		return "eq"
	case OperatorNe:
		return "ne"
	case OperatorExists:
		return "exists"
	case OperatorGt:
		return "gt"
	case OperatorGte:
		return "gte"
	case OperatorLt:
		return "lt"
	case OperatorLte:
		return "lte"
	case OperatorBetween:
		return "between"
	case OperatorBetweenEx:
		return "between_exc"
	case OperatorBetweenExInc:
		return "between_exc_inc"
	case OperatorBetweenIncEx:
		return "between_inc_exc"
	case OperatorLike:
		return "like"
	case OperatorNotLike:
		return "notlike"
	case OperatorILike:
		return "ilike"
	case OperatorIn:
		return "in"
	case OperatorNotIn:
		return "not in"
	case OperatorWeekdayIn:
		return "weekday_in"
	case OperatorTimeOfDayBetween:
		return "tod_between"
	case OperatorUnspecified:
		return strUnspecified
	default:
		return ""
	}
}
