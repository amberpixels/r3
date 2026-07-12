// Package r3pq is a raw-SQL r3.CRUD[T, ID] driver backed by lib/pq
// (github.com/lib/pq) over database/sql.
//
// It builds SQL by reflection and scans into structs tagged `db:"col,pk"`; no
// ORM, so no preloads and no soft-delete (IncludeTrashed, Restore, HardDelete are
// unavailable). Table names derive from the struct name (CamelCase -> snake_case,
// pluralized); nullable columns need pointer fields. The r3 SQL dialect emits `?`
// placeholders, converted to Postgres `$1, $2, ...` internally. Transactions via
// r3.Transactor; reach the *sql.DB through Raw().DB for CTEs and the like.
package r3pq
