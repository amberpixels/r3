// Package r3gopg provides an r3.CRUD[T, ID] driver backed by go-pg v10,
// a PostgreSQL ORM with a focus on PostgreSQL-specific features.
//
// Driver: github.com/go-pg/pg/v10
// Source: https://github.com/go-pg/pg
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Preloads via go-pg's Relation() (has-one, has-many, many-to-many)
//   - IncludeTrashed via go-pg's AllWithDeleted() (requires pg:",soft_delete" tag)
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (GoPgRaw) for custom go-pg orm.Query usage
//
// Limitations / notes:
//   - go-pg is in maintenance mode; the authors recommend migrating to Bun.
//     See: https://github.com/go-pg/pg#bun
//   - go-pg does NOT wrap database/sql; it uses its own connection pool (*pg.DB).
//     If you need a *sql.DB (e.g. for goose migrations), open a separate connection.
//   - Model structs must use `pg` struct tags (e.g. `pg:"column_name"`).
//   - Aggregate / custom-shape queries should use Raw().Scan() into a dedicated struct,
//     since Raw().Find() scans into []T and go-pg rejects unknown columns.
package r3gopg
