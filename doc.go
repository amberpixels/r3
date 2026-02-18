// Package r3 provides a universal, database-agnostic CRUD repository abstraction for Go.
//
// The core interface is [CRUD], a generic interface parameterized by entity type T
// and primary key type ID. It composes [Querier] (Get, List) and [Commander]
// (Create, Update, Patch, Delete) — use the narrower sub-interfaces when full
// CRUD access is not needed (e.g. read-only config stores only need [Querier]).
//
// Queries are built using composable types: [Filters], [Sorts], [PaginationSpec],
// [Fields], and [Preloads], all combined into a single [Query] value.
//
// Database-specific drivers (in the drivers/ subdirectory) implement CRUD for
// PostgreSQL, MySQL, SQLite, MongoDB, and ORMs like GORM and Bun. The engine/
// subdirectory provides shared base implementations that drivers embed.
//
// Optional features like change history tracking are available in the features/
// subdirectory.
package r3
