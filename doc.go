// Package r3 provides a universal, backend-agnostic CRUD repository abstraction
// for Go: the same [CRUD] interface over PostgreSQL, MySQL, SQLite, MongoDB, and
// JSON/YAML/TOML files. Business code depends on r3, never on a concrete backend.
//
// [CRUD] is generic over entity type T and primary key ID, composing [Querier]
// (Get, List, Count) and [Commander] (Create, Update, Patch, Delete). Depend on
// the narrower sub-interface when that is all you need (a read-only store needs
// only [Querier]).
//
// Queries are immutable value types — [Filters], [Sorts], [PaginationSpec],
// [CursorSpec], [Fields], [Preloads] — assembled into one [Query]. [Query.MergeWith]
// returns a new value, so queries from different sources (defaults, request,
// permission scope) combine without mutation. Build filters with the short-form
// helpers ([Eq], [Gt], [In], [Like], [Between], ...) or, for table hints and
// nested paths, the [FieldSpec] forms ([F], [Fop]).
//
// # Schema and capabilities
//
// [SchemaOf] reflects an entity's struct tags into a [Schema] of
// capability-bearing [Attribute]s. Five capabilities gate what a field may do:
// [Filterable], [Sortable], [Queryable], [Creatable], [Mutable]. Defaults are
// permissive (a plain scalar gets all five); tags only tighten them (no-filter,
// no-sort, no-output, readonly, immutable, enum). The PK, created_at/updated_at,
// and the soft-delete column are read-only by default.
//
// Capabilities are the public ceiling. [Schema.ValidateQuery] rejects an unknown
// or disallowed field with a typed error ([ErrUnknownField], [ErrFieldNotFilterable],
// [ErrFieldNotSortable], [ErrFieldNotQueryable]) before any SQL is built, and the
// engine shapes writes to honor Creatable/Mutable so a full Update cannot clobber
// created_at or resurrect a soft-deleted row. created_at/updated_at are
// system-managed: read-only to callers but stamped with server time by the engine.
// The permissions feature only narrows the ceiling per-actor.
//
// [WithoutWriteGuard] (on the context) and the [SystemWriter] wrapper are the
// audited system/worker door: they skip the capability check but not the
// structural floor (the PK and computed attributes stay unwritable), and
// history/metrics still record the write.
//
// # Errors and pagination
//
// Get normalizes every backend's not-found condition to [ErrNotFound], so
// errors.Is works the same across drivers. List paginates by default
// ([PageSizeDefault] items) unless passed [Unpaginated]; compare the returned
// total against the slice length to detect truncation. Count answers "how many
// match?" without materializing rows (only Filters and IncludeTrashed apply).
//
// # Aggregation
//
// [Aggregator] is the opt-in grouped-aggregation capability: GROUP BY plus
// COUNT/COUNT DISTINCT/SUM/AVG/MIN/MAX over the matching records, returning
// [AggregateRow] values. Declare the shape on the Query ([Query.GroupBy],
// [Query.Aggregates], [Query.Having]) via [GroupBy], [AggCount], [AggSum],
// [AggMin], [AggMax], [AggAvg], [AggCountDistinct], and reach it with
// [AggregateOf]. Every engine implements it and every decorator forwards it, so
// it survives decoration.
//
//	rows, err := r3.AggregateOf(ctx, raidRepo, r3.Query{
//	    GroupBy:    r3.GroupBy("location_id"),
//	    Aggregates: r3.Aggregates{r3.AggCount("raids"), r3.AggMax("date", "last_raid")},
//	})
//
// # Relationships
//
// Relations (has-many, belongs-to, many-to-many) are declared either by struct
// tag (`r3:"rel:has-many,fk:city_id"`) or physically by table and column via
// [WithRelations] with [HasManyRelation], [BelongsToRelation], and
// [ManyToManyRelation]. The physical form lets an entity relate to a table it
// does not import as a Go type, avoiding domain import cycles. A declared
// relation supports three operations through the normal Query helpers:
//
//   - [Has] — relationship filter (EXISTS): owners with at least one related row
//     matching the inner filters.
//   - [HasNo] — its negation (NOT EXISTS / anti-join), correctly including owners
//     with a NULL foreign key.
//   - [AggregateThroughRelation] — grouped aggregation over the related rows, via
//     [RelationAggregator]. Owner filters (including a Scoper's) restrict which
//     owners fold in, and soft-deleted related rows are excluded.
//
// Relation resolution is GORM-only today; other SQL drivers reject relationship
// filters and the mongo/file engines ignore them (tracked in docs/backend-parity.md).
//
// # Project layout
//
// Five layers, each depending only on the layers above:
//
//   - r3 (this package): interfaces and the immutable query model. Zero deps.
//   - dialects/: pure, stateless converters between r3 types and a format — data-store
//     (sql, bson), serialization (json, yaml, toml, url, schema), and the when bridge
//     that compiles human time vocabulary into recurring time-pattern filters. No I/O.
//   - engine/: complete CRUD per storage category (sql, mongo, file); reflection,
//     query building, execution.
//   - drivers/: ready-to-use constructors for a specific library.
//   - features/: decorators (permissions, history, metrics, validation, i18n,
//     softdelete, transactor) that wrap any r3.CRUD regardless of backend.
//
// Features compose across backends: the same permission checker, audit log, or
// metrics collector works whether the repo is PostgreSQL, MongoDB, or a YAML file
// — the behavioral layer is written once and applied everywhere.
package r3
