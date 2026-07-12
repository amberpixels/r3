package r3sql

// SQLClauseOperator represents a SQL operator. e.g. "=", "LIKE", "IN".
type SQLClauseOperator string

// List of supported SQL operators.
const (
	SQLClauseOperatorEq      SQLClauseOperator = "="
	SQLClauseOperatorNe      SQLClauseOperator = "!="
	SQLClauseOperatorGt      SQLClauseOperator = ">"
	SQLClauseOperatorGte     SQLClauseOperator = ">="
	SQLClauseOperatorLt      SQLClauseOperator = "<"
	SQLClauseOperatorLte     SQLClauseOperator = "<="
	SQLClauseOperatorLike    SQLClauseOperator = "LIKE"
	SQLClauseOperatorNotLike SQLClauseOperator = "NOT LIKE"
	SQLClauseOperatorILike   SQLClauseOperator = "ILIKE"
	SQLClauseOperatorIn      SQLClauseOperator = "IN"
	SQLClauseOperatorNotIn   SQLClauseOperator = "NOT IN"
)

// String implements fmt.Stringer.
func (op SQLClauseOperator) String() string { return string(op) }

// ToSQL returns the operator as it appears in generated SQL.
func (op SQLClauseOperator) ToSQL() string { return string(op) }
