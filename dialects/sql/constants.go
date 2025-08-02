package r3sql

// SQL Keywords and Operators.
const (
	// Logical operators.
	sqlAnd = "AND"
	sqlOr  = "OR"

	// Sort directions.
	sqlAsc  = "ASC"
	sqlDesc = "DESC"

	// Nulls position.
	sqlNullsFirst = "NULLS FIRST"
	sqlNullsLast  = "NULLS LAST"

	// Null comparisons.
	sqlIsNull    = "IS NULL"
	sqlIsNotNull = "IS NOT NULL"

	// Comparison operators (already defined in filter-operator.go but for reference)
	// sqlEqual     = "="
	// sqlNotEqual  = "!="
	// sqlLike      = "LIKE"
	// sqlNotLike   = "NOT LIKE"
	// sqlILike     = "ILIKE"
	// sqlIn        = "IN"
	// sqlNotIn     = "NOT IN"
	// sqlGt        = ">"
	// sqlGte       = ">="
	// sqlLt        = "<"
	// sqlLte       = "<=".
)
