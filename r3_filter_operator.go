package r3

// FilterOperatorSpec represents an operator to be used in a filter.
// For now, not all r3 operators are supported by every possible dialect.
// But the idea is to provide here in r3 the most possibly full list of operators that we need.
// TODO: might be refactored into a more complex struct
type FilterOperatorSpec int8

const (
	OperatorUnspecified  FilterOperatorSpec = iota
	OperatorEq                              // =
	OperatorNe                              // !=
	OperatorExists                          // exists
	OperatorGt                              // >
	OperatorGte                             // >=
	OperatorLt                              // <
	OperatorLte                             // <=
	OperatorBetween                         // between_inc meaning []
	OperatorBetweenEx                       // between_exc meaning ()
	OperatorBetweenExInc                    // between_exc_inc meaning (]
	OperatorBetweenIncEx                    // between_inc_exc meaning [)
	OperatorIn                              // in
	OperatorNotIn                           // not in
	OperatorLike                            // like
	OperatorNotLike                         // not like
	OperatorILike                           // ilike (like + case insensitive)
)

// String is implemented for debugging purposes, so the FilterOperatorSpec is a fmt.Stringer.
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
	case OperatorUnspecified:
		return "unspecified"
	default:
		return ""
	}
}
