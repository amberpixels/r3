package r3bson

// BSONOperator represents a MongoDB query operator (e.g. "$eq", "$regex", "$in").
type BSONOperator string

// List of supported BSON operators.
const (
	BSONOperatorEq      BSONOperator = "$eq"
	BSONOperatorNe      BSONOperator = "$ne"
	BSONOperatorGt      BSONOperator = "$gt"
	BSONOperatorGte     BSONOperator = "$gte"
	BSONOperatorLt      BSONOperator = "$lt"
	BSONOperatorLte     BSONOperator = "$lte"
	BSONOperatorIn      BSONOperator = "$in"
	BSONOperatorNin     BSONOperator = "$nin"
	BSONOperatorRegex   BSONOperator = "$regex"
	BSONOperatorExists  BSONOperator = "$exists"
	BSONOperatorOptions BSONOperator = "$options"
	BSONOperatorNot     BSONOperator = "$not"
	BSONOperatorAnd     BSONOperator = "$and"
	BSONOperatorOr      BSONOperator = "$or"
)

// String so we implement fmt.Stringer.
func (op BSONOperator) String() string { return string(op) }
