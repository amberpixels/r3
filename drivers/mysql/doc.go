// Package r3mysql provides an r3.CRUD[T, ID] driver backed by go-sql-driver/mysql,
// the pure Go MySQL driver for database/sql.
//
// Driver: github.com/go-sql-driver/mysql
// Source: https://github.com/go-sql-driver/mysql
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
//   - No soft-delete support. IncludeTrashed is ignored.
//   - Table names are derived automatically from struct name (CamelCase -> snake_case + plural).
//   - Nullable columns require pointer types (e.g. *string, *int64) in the model struct.
//   - Uses `?` placeholders natively (no conversion needed).
//   - MySQL does not support RETURNING; Create uses INSERT + LAST_INSERT_ID() + SELECT
//     to return the full created entity including auto-generated columns.
//   - ILIKE is not supported by MySQL; use LIKE instead (case sensitivity depends on
//     column collation; default utf8mb4_0900_ai_ci is case-insensitive).
//   - NULLS FIRST/LAST is not natively supported by MySQL.
//   - DSN must include `parseTime=true` for time.Time scanning to work correctly.
//   - Transactions are supported via the r3.Transactor interface (BeginTx).
//   - For advanced use cases (CTEs, etc.), use Raw().DB to access
//     the underlying *sql.DB directly.
package r3mysql
