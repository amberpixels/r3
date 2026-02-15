// Package r3pgx provides an r3.CRUD[T, ID] driver backed by jackc/pgx,
// the pure Go PostgreSQL driver.
//
// Driver: github.com/jackc/pgx/v5
// Source: https://github.com/jackc/pgx
//
// This driver uses pgx in database/sql compatibility mode (pgx/v5/stdlib),
// which means it works through the standard database/sql interface while
// still benefiting from pgx's connection handling and type support.
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (BaseRaw) for arbitrary SQL with $1-style placeholders
//
// Limitations / notes:
//   - No ORM layer: this driver builds raw SQL and uses reflection-based struct scanning.
//     Model structs must use `db` struct tags (e.g. `db:"column_name,pk"`).
//   - No preload support. Relations (joins, eager loading) must be done via Raw().
//   - No soft-delete support. IncludeTrashed is ignored.
//   - Table names are derived automatically from struct name (CamelCase -> snake_case + plural).
//   - Nullable columns require pointer types (e.g. *string, *int64) in the model struct.
//   - The r3 SQL dialect produces `?` placeholders; this driver converts them to
//     PostgreSQL-style `$1, $2, ...` numbered placeholders internally.
//   - Transactions are supported via the r3.Transactor interface (BeginTx).
//   - For advanced use cases (CTEs, etc.), use Raw().DB to access
//     the underlying *sql.DB directly.
package r3pgx
