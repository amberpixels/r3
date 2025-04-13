package r3sql

import (
	"github.com/amberpixels/r3"
)

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

	// TODO: custom support for Between, NotBetween, etc
)

func (op SQLClauseOperator) String() string { return string(op) }
func (op SQLClauseOperator) ToSQL() string  { return string(op) }

// SQLOperatorDialector implements filter.FilterOperatorDialector for SQL dialect.
type SQLOperatorDialector struct{}

var _ r3.FilterOperatorDialector = (*SQLOperatorDialector)(nil)

func (v *SQLOperatorDialector) Eq() r3.DialectValue      { return SQLClauseOperatorEq }
func (v *SQLOperatorDialector) Ne() r3.DialectValue      { return SQLClauseOperatorNe }
func (v *SQLOperatorDialector) Gt() r3.DialectValue      { return SQLClauseOperatorGt }
func (v *SQLOperatorDialector) Gte() r3.DialectValue     { return SQLClauseOperatorGte }
func (v *SQLOperatorDialector) Lt() r3.DialectValue      { return SQLClauseOperatorLt }
func (v *SQLOperatorDialector) Lte() r3.DialectValue     { return SQLClauseOperatorLte }
func (v *SQLOperatorDialector) Like() r3.DialectValue    { return SQLClauseOperatorLike }
func (v *SQLOperatorDialector) NotLike() r3.DialectValue { return SQLClauseOperatorNotLike }
func (v *SQLOperatorDialector) ILike() r3.DialectValue   { return SQLClauseOperatorILike }
func (v *SQLOperatorDialector) In() r3.DialectValue      { return SQLClauseOperatorIn }
func (v *SQLOperatorDialector) NotIn() r3.DialectValue   { return SQLClauseOperatorNotIn }

func (v *SQLOperatorDialector) Exists() r3.DialectValue       { panic("not implemented") }
func (v *SQLOperatorDialector) Between() r3.DialectValue      { panic("not implemented") }
func (v *SQLOperatorDialector) BetweenEx() r3.DialectValue    { panic("not implemented") }
func (v *SQLOperatorDialector) BetweenIncEx() r3.DialectValue { panic("not implemented") }
func (v *SQLOperatorDialector) BetweenExInc() r3.DialectValue { panic("not implemented") }
