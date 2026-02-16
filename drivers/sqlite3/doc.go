// Package r3sqlite3 provides an r3.CRUD[T, ID] driver backed by mattn/go-sqlite3,
// the CGo SQLite3 driver for database/sql.
//
// Driver: github.com/mattn/go-sqlite3
// Source: https://github.com/mattn/go-sqlite3
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (BaseRaw) for arbitrary SQL with ?-style placeholders
//
// Limitations / notes:
//   - No ORM layer: this driver builds raw SQL and uses reflection-based struct scanning.
//     Model structs must use `db` struct tags (e.g. `db:"column_name,pk"`).
//   - No preload support. Relations (joins, eager loading) must be done via Raw().
//   - No soft-delete support. IncludeTrashed is ignored. Restore/HardDelete not available.
//   - Table names are derived automatically from struct name (CamelCase -> snake_case + plural).
//   - Nullable columns require pointer types (e.g. *string, *int64) in the model struct.
//   - Uses `?` placeholders natively (no conversion needed).
//   - ILIKE is not supported by SQLite; use LIKE instead (SQLite LIKE is case-insensitive
//     for ASCII characters by default).
//   - RETURNING clause requires SQLite 3.35+ (supported by go-sqlite3).
//   - Transactions are supported via the r3.Transactor interface (BeginTx).
//   - For advanced use cases (CTEs, etc.), use Raw().DB to access
//     the underlying *sql.DB directly.
package r3sqlite3
