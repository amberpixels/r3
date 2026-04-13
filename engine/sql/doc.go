// Package enginesql provides the SQL engine for r3.
//
// It serves two roles:
//
//  1. Full CRUD via [BaseCRUD]: a generic r3.CRUD implementation using database/sql
//     and reflection-based struct scanning. Raw SQL drivers (pq, pgx, mysql, sqlite3)
//     embed BaseCRUD and only configure it via [Flavor] (placeholder style,
//     RETURNING support, timestamp function).
//
//  2. Shared query preparation via [PreparedListQuery]: ORM-based drivers (GORM, Bun,
//     go-pg) do NOT embed BaseCRUD — they use their own ORM API for query execution.
//     However, they share PreparedListQuery to convert r3 filters, sorts, and pagination
//     into SQL-ready components, avoiding duplicated translation logic.
//
// Key types:
//   - [BaseCRUD]: full r3.CRUD backed by database/sql
//   - [BaseRaw]: escape hatch for arbitrary SQL queries
//   - [StructMeta]: reflection-based struct metadata (columns, PK, relations, soft-delete)
//   - [Flavor]: database-specific SQL variations (placeholders, RETURNING, timestamps)
//   - [PreparedListQuery]: pre-computed SQL clauses from an r3.Query, shared by all SQL drivers
//
// This package is public so that third-party drivers can reuse it.
package enginesql
