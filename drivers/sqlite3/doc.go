// Package r3sqlite3 is a raw-SQL r3.CRUD[T, ID] driver for SQLite over
// database/sql. It works with any database/sql SQLite driver and imports none
// itself: blank-import one and pass the *sql.DB. The two known ones are
// github.com/mattn/go-sqlite3 (CGo, registers "sqlite3") and modernc.org/sqlite
// (pure Go, registers "sqlite"; builds with CGO_ENABLED=0).
//
// It builds SQL by reflection and scans into structs tagged `db:"col,pk"`; no
// ORM, so no preloads and no soft-delete (IncludeTrashed, Restore, HardDelete are
// unavailable). Table names derive from the struct name (CamelCase -> snake_case,
// pluralized); nullable columns need pointer fields. Transactions via
// r3.Transactor; reach the *sql.DB through Raw().DB for CTEs and the like.
//
// SQLite-specific notes:
//   - No ILIKE (use LIKE; SQLite LIKE is case-insensitive for ASCII by default).
//   - RETURNING requires SQLite 3.35+ (both drivers above bundle a newer one).
package r3sqlite3
