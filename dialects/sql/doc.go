// Package r3sql translates r3 query types into SQL clauses.
//
// Category: Data store dialect.
//
// It converts [r3.Filters] to WHERE clauses with positional placeholders,
// [r3.Sorts] to ORDER BY expressions, and [r3.PaginationSpec] to LIMIT/OFFSET.
// This package is used by the SQL engine and drivers; most users do not
// import it directly.
package r3sql
