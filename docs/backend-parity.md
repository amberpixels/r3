# Backend feature parity

R3's promise is *universal query parity* - the same `r3.Query` behaves
consistently across backends wherever the underlying store supports it. In
practice some capabilities land on one backend first (usually GORM, our first
real consumer's backend) and have not yet been brought to the others.

This document is the **single tracked list of those gaps**. When a capability is
implemented for a new backend, move it up to "At parity" (or delete the row when
every backend supports it). When a new backend-specific capability is added,
record its gap here in the same PR.

Legend: ✅ supported · ⚠️ degrades gracefully (documented) · ❌ not implemented
(errors or ignored).

Closing the Mongo column is planned and sequenced in
[`plan-mongo-parity.md`](./plan-mongo-parity.md) (a MongoDB consumer is driving
it).

## Known gaps (GORM ahead of the rest)

| Capability | GORM | engine/sql raw drivers (pq, pgx, mysql, sqlite3) | bun / gopg | mongo | file |
|---|---|---|---|---|---|
| Relationship filters - `r3.Has` (EXISTS) | ✅ | ❌ errors at SQL translation | ❌ | ❌ ignored | ❌ ignored |
| Relationship filters - `r3.HasNo` (anti-join / NOT EXISTS) | ✅ | ❌ | ❌ | ❌ | ❌ |
| Explicitly-declared relations - `r3.WithRelations` (table+column) | ✅ | ❌ | ❌ | ❌ | ❌ |
| Relation aggregation - `r3.AggregateThroughRelation` | ✅ | ❌ `ErrRelationAggregateNotSupported` | ❌ | ❌ | ❌ |
| Value codecs - `r3:"codec:…"` reads/writes + filter args | ✅ | ❌ `RequireCodecSupport` panics at construction | ❌ panics | ❌ panics | ❌ panics |
| Value codecs - aggregate `min`/`max` decode on a codec'd field | ✅ | ❌ | ❌ | ❌ | ❌ |

Value codecs are the transparent Go-value ⇄ stored-value transform (flagship
`time.Time` ⇄ unix int). Design + rollout status live in
[`plan-field-codecs.md`](./plan-field-codecs.md). Every not-yet-wired backend
**panics at construction** (`r3.RequireCodecSupport`) rather than silently
storing the un-encoded value, so a declared codec can never corrupt data on an
unsupported backend. Filter/cursor argument encoding is already shared in
`engine/sql.PrepareMergedListQuerySchema`, so porting to the raw SQL drivers is
mostly wiring the scan/bind path.

Why GORM-only today: relationship resolution is implemented as a **pre-query key
set lowering** in `drivers/gorm/relfilter.go` (a `Has` becomes an `IN`, a `HasNo`
a `NOT IN`) plus a join/child-table aggregate in `drivers/gorm/relaggregate.go`.
Nothing about the approach is GORM-specific - it uses the shared
`engine/sql.RelationMeta` metadata and the pure `dialects/sql` builders - so
porting it is mostly wiring the same lowering into `engine/sql.BaseCRUD` (which
the raw SQL drivers embed) and adding native/emulated forms for mongo and file.

The core types are already backend-agnostic:

- `r3.RelationSpec` + builders (`HasManyRelation`, `BelongsToRelation`,
  `ManyToManyRelation`) and `r3.WithRelations` live in core.
- `r3.Has` / `r3.HasNo` are ordinary `FilterSpec`s (`Relation`,
  `RelationFilter`, `RelationNegate`).
- `r3.RelationAggregator` / `r3.AggregateThroughRelation` mirror
  `Aggregator` / `AggregateOf` and are forwarded by every feature decorator.

So closing a gap is a driver/engine change only - no core API churn.

## Related still-open relation work

These are feature gaps (not backend-parity gaps) tracked in the p44 feedback log
(`R3_TODO.md`), listed here because they touch the same subsystem:

- **R3-011 (join-table aggregation)** - *shipped* for has-many child tables and
  many-to-many join tables. Aggregating *target* columns through an M2M (e.g.
  `MAX(person.age)` per group) is not yet supported: aggregate/group columns
  resolve against the related base table, and the target is joined only to
  exclude soft-deleted rows.
- **R3-012 (i18n overlay on preloads)** - unrelated to relation resolution;
  still open.

## Already at parity (for reference)

`Aggregator` (single-table `GROUP BY`), `Count`, `ErrNotFound` normalization,
`Upserter` and `BulkPatcher` (GORM + engine/sql raw drivers; mongo/file/bun/gopg
degrade to their respective `Err…NotSupported`). See each capability's doc
comment for the authoritative per-backend status.
