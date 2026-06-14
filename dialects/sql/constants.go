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

	// Constant predicates, used for degenerate (empty-set) IN / NOT IN clauses.
	// `IN ()` is invalid SQL on every backend, so an empty set collapses to a
	// constant: `x IN ()` matches nothing, `x NOT IN ()` matches everything.
	// `1=0` / `1=1` are portable across Postgres, MySQL and SQLite.
	sqlFalse = "1=0"
	sqlTrue  = "1=1"

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
