// Package r3 provides a universal CRUD repository abstraction for Go.
//
// The core interface is [CRUD], a generic interface parameterized by entity type T
// and primary key type ID. It composes [Querier] (Get, List, Count) and [Commander]
// (Create, Update, Patch, Delete) — use the narrower sub-interfaces when full
// CRUD access is not needed (e.g. read-only config stores only need [Querier]).
//
// Queries are built with composable value types — [Filters], [Sorts],
// [PaginationSpec], [CursorSpec], [Fields], and [Preloads] — combined into a
// single [Query] struct. Queries are immutable; [Query.MergeWith] returns a new
// value, making it safe to combine queries from different sources (defaults,
// user request, permission scope). Build filters with the short-form helpers
// ([Eq], [Gt], [In], [Like], [Between], ...) for the common case, or the
// [FieldSpec]-based forms ([F], [Fop]) when you need table hints or nested paths.
//
// # Errors and pagination
//
// Get normalizes every backend's "not found" condition to the [ErrNotFound]
// sentinel, so callers detect a missing record with errors.Is the same way
// regardless of driver. List paginates by default ([PageSizeDefault] items);
// pass [Unpaginated] to opt out, or compare the returned total against the slice
// length to detect truncation. Count answers "how many match?" without
// materializing rows — only Filters and IncludeTrashed affect its result.
//
// # Project layout
//
// The project is organized in five layers, each depending only on the layers above:
//
//   - r3 (this package): Interfaces and query model. Zero external dependencies.
//   - dialects/: Stateless converters between r3 types and format-specific
//     representations. Two categories: data-store (sql, bson) and serialization
//     (json, yaml, toml, url). No I/O.
//   - engine/: Complete CRUD implementations per storage category (sql, mongo, file).
//     Contains reflection, query building, and execution logic.
//   - drivers/: Ready-to-use constructors for specific libraries. Raw SQL drivers
//     (pq, pgx, mysql, sqlite3) embed engine/sql.BaseCRUD. ORM drivers (gorm, bun,
//     gopg) use their own ORM API but share engine/sql.PreparedListQuery for
//     filter/sort/pagination translation. The mongo driver wraps engine/mongo.
//   - features/: Composable decorators (permissions, history, metrics, validation,
//     softdelete, transactor) that wrap any r3.CRUD regardless of backend.
//
// # Key design principle
//
// Features compose across backends. The same permission checker, audit log, or
// metrics collector works whether the underlying repo is PostgreSQL, MongoDB,
// or a YAML file. This makes r3 particularly useful in systems that use multiple
// data sources — the behavioral layer is written once and applied everywhere.
package r3
