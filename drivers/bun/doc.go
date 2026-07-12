// Package r3bun is an r3.CRUD[T, ID] driver backed by Bun (github.com/uptrace/bun),
// a SQL-first ORM for PostgreSQL, MySQL, MSSQL, and SQLite. It maps preloads onto
// Bun's Relation(), IncludeTrashed onto WhereAllWithDeleted() (needs soft-delete
// model setup), and exposes Restore/HardDelete.
//
// Notes:
//   - Models must embed bun.BaseModel (with a table tag) and use `bun` tags.
//   - Bun wraps database/sql natively; use db.DB for goose migrations or raw usage.
//   - For aggregate / custom-shape rows use Raw().Scan() into a dedicated struct;
//     Raw().Find() scans into []T and Bun rejects unknown columns.
package r3bun
