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

	// Constant predicates for degenerate (empty-set) IN / NOT IN clauses, since
	// `IN ()` is invalid SQL everywhere: `x IN ()` matches nothing, `x NOT IN ()`
	// matches everything. `1=0` / `1=1` are portable across Postgres/MySQL/SQLite.
	sqlFalse = "1=0"
	sqlTrue  = "1=1"

	// Comparison operators live in filter_operator.go.
)
