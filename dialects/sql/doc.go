// Package r3sql translates r3 query types into SQL clauses: [r3.Filters] to
// WHERE with positional placeholders, [r3.Sorts] to ORDER BY, [r3.PaginationSpec]
// to LIMIT/OFFSET. Used by the SQL engine and drivers; rarely imported directly.
//
// DB-specific variations (ILIKE, placeholder style, NULLS FIRST/LAST) are the
// engine layer's job via engine/sql.Flavor; this package emits generic SQL.
package r3sql
