// Package r3mysql is a raw-SQL r3.CRUD[T, ID] driver backed by go-sql-driver/mysql
// (github.com/go-sql-driver/mysql) over database/sql.
//
// It builds SQL by reflection and scans into structs tagged `db:"col,pk"`; no
// ORM, so no preloads and no soft-delete (IncludeTrashed, Restore, HardDelete are
// unavailable). Table names derive from the struct name (CamelCase -> snake_case,
// pluralized); nullable columns need pointer fields. Transactions via
// r3.Transactor; reach the *sql.DB through Raw().DB for CTEs and the like.
//
// MySQL-specific notes:
//   - No RETURNING: Create uses INSERT + LAST_INSERT_ID() + SELECT to return the
//     full row including auto-generated columns.
//   - No ILIKE (use LIKE; case sensitivity follows the column collation, and the
//     default utf8mb4_0900_ai_ci is case-insensitive) and no NULLS FIRST/LAST.
//   - The DSN must set `parseTime=true` for time.Time scanning.
package r3mysql
