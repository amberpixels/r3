// Package r3gopg is an r3.CRUD[T, ID] driver backed by go-pg v10
// (github.com/go-pg/pg/v10), a PostgreSQL ORM. It maps preloads onto go-pg's
// Relation(), IncludeTrashed onto AllWithDeleted() (needs a pg:",soft_delete"
// tag), and exposes Restore/HardDelete. Map fields with `pg` tags.
//
// Notes:
//   - go-pg is in maintenance mode; the authors recommend Bun (see
//     https://github.com/go-pg/pg#bun).
//   - go-pg does NOT wrap database/sql; it owns its pool (*pg.DB). For a *sql.DB
//     (e.g. goose migrations), open a separate connection.
//   - For aggregate / custom-shape rows use Raw().Scan() into a dedicated struct;
//     Raw().Find() scans into []T and go-pg rejects unknown columns.
package r3gopg
