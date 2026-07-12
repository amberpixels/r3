// Package r3sqlite3 is a raw-SQL r3.CRUD[T, ID] driver backed by mattn/go-sqlite3
// (github.com/mattn/go-sqlite3, CGo) over database/sql.
//
// It builds SQL by reflection and scans into structs tagged `db:"col,pk"`; no
// ORM, so no preloads and no soft-delete (IncludeTrashed, Restore, HardDelete are
// unavailable). Table names derive from the struct name (CamelCase -> snake_case,
// pluralized); nullable columns need pointer fields. Transactions via
// r3.Transactor; reach the *sql.DB through Raw().DB for CTEs and the like.
//
// SQLite-specific notes:
//   - No ILIKE (use LIKE; SQLite LIKE is case-insensitive for ASCII by default).
//   - RETURNING requires SQLite 3.35+ (which go-sqlite3 provides).
package r3sqlite3
