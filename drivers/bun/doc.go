// Package r3bun provides an r3.CRUD[T, ID] driver backed by Bun,
// a SQL-first Go ORM for PostgreSQL, MySQL, MSSQL, and SQLite.
//
// Driver: github.com/uptrace/bun
// Source: https://github.com/uptrace/bun
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Preloads via Bun's Relation() (belongs-to, has-one, has-many, many-to-many)
//   - IncludeTrashed via Bun's WhereAllWithDeleted() (requires soft-delete model setup)
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (BunRaw) for custom bun.SelectQuery usage
//
// Limitations / notes:
//   - Bun wraps database/sql natively; use db.DB (the underlying *sql.DB) for goose
//     migrations or any raw database/sql usage.
//   - Model structs must embed bun.BaseModel with a table tag and use `bun` struct tags.
//   - Aggregate / custom-shape queries should use Raw().Scan() into a dedicated struct,
//     since Raw().Find() scans into []T and Bun rejects unknown columns.
//   - Bun is the recommended successor to go-pg by the same authors.
package r3bun
