// Package enginesql is r3's SQL engine, serving two roles:
//
//  1. Full CRUD via [BaseCRUD]: a generic r3.CRUD over database/sql with
//     reflection-based struct scanning. Raw SQL drivers (pq, pgx, mysql, sqlite3)
//     embed it and configure only a [Flavor].
//
//  2. Shared query preparation via [PreparedListQuery]: ORM drivers (GORM, Bun,
//     go-pg) execute through their own API but reuse this to translate r3 filters,
//     sorts, and pagination into SQL-ready pieces instead of duplicating it.
//
// Key types: [BaseCRUD], [BaseRaw] (arbitrary-query escape hatch), [StructMeta]
// (reflected columns/PK/relations/soft-delete), [Flavor], [PreparedListQuery].
//
// Public so third-party drivers can reuse it.
package enginesql
