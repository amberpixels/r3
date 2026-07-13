# Mongo backend parity - plan & rollout

Bringing `engine/mongo` (and its `drivers/mongo` wrapper) up to the capabilities
GORM already has, driven by a new consumer whose primary store is MongoDB.

This is the sequenced, actionable companion to the parity matrix in
[`backend-parity.md`](./backend-parity.md): that file tracks *what* the gaps are,
this file tracks *how* to close the Mongo ones and in *what order*. Update both
when an increment lands (remove the Mongo column's ❌ in the matrix, tick the box
here).

Verified against the current Mongo code, not just the docs. What Mongo already
supports and needs no work: base CRUD, single-table `Aggregator` (`$group`),
`Count`, `ErrNotFound` normalization, offset **and** cursor/keyset pagination,
`Preloads` (`$in`), and soft-delete.

## Scope & priority

Consumer-driven order (highest first):

1. **Upsert + BulkPatch** - quickest win, both map to native Mongo ops.
2. **Value codecs** (`r3:"codec:…"`) - the hard one; blocks the consumer's
   `time.Time` fields today.
3. **Relations** - `Has` / `HasNo` filters, `WithRelations`, and
   `AggregateThroughRelation`.

**Transactions** (`Transactor` / `BeginTx` / `InTx`) are the next step after
these land - on the table, not this increment. Mongo supports multi-document
transactions via sessions + `WithTransaction`, which needs a replica set.

## 1. Upsert + BulkPatch

Neither interface is implemented on Mongo today, so `r3.UpsertOf` /
`r3.PatchWhereOf` return `ErrUpsertNotSupported` / `ErrBulkPatchNotSupported`.
Both have direct Mongo equivalents, so this is the cheapest increment.

- **Upsert** (`r3.Upserter`): `Upsert(ctx, entity, opts...) (T, error)`. Map to
  `UpdateOne`/`ReplaceOne` with `SetUpsert(true)`. The conflict target is the PK
  by default (`_id`); honor `UpsertSpec.ConflictColumns` (a unique-index field
  set) and `UpsertSpec.UpdateFields` (empty = replace all mutable columns) by
  shaping the `$set`. Reference the SQL semantics in `engine/sql/upsert.go`.
- **BulkPatch** (`r3.BulkPatcher`): `PatchWhere(ctx, filters, entity, fields)
  (int64, error)`. Map to `UpdateMany` with the filter from
  `r3bson.FiltersToBSON` and a `$set` of `fields`; return `ModifiedCount`.
  Reference `engine/sql/bulkpatch.go`.

Checklist:
- [x] Implement `Upsert` on `engine/mongo.BaseCRUD`; add `var _ r3.Upserter[...]`.
- [x] Implement `PatchWhere` on `engine/mongo.BaseCRUD`; add `var _ r3.BulkPatcher[...]`.
- [x] Forward both through `drivers/mongo.MongoCRUD` (promoted via embedding).
- [x] Tests (testcontainers): upsert insert vs update branch, conflict target,
      partial `UpdateFields`; bulk patch affected-count and filter scoping.
- [x] `backend-parity.md`: move Upsert/BulkPatch off the Mongo degrade list.

## 2. Value codecs (`r3:"codec:…"`)

Today every Mongo repo constructor calls `r3.RequireCodecSupport(...)`
(`engine/mongo/crud.go`), which **panics** if any attribute declares a codec - so
a codec'd entity cannot build a Mongo repo at all. The guard exists so a declared
codec never silently stores an un-encoded value; it comes off only when all four
paths below apply the codec.

The four paths a codec touches, and Mongo's status on each:

- **Write** - clean choke-point. Entities serialize through
  `StructMeta.ToBSONDoc` (`engine/mongo/reflect.go:325`, called from
  `crud.go`). Apply `Codec.Encode` per codec'd attribute there.
- **Filter / cursor args** - already solved in core. `r3.EncodeFilterCodecs` /
  `r3.EncodeCursorCodecs` encode a domain value to stored form; wire them into
  Mongo's list prep the way `engine/sql.PrepareMergedListQuerySchema` does
  (`engine/sql/list_query.go:66`), before `r3bson.FiltersToBSON` /
  `CursorToBSON`.
- **Read - the hard blocker.** `FindOne().Decode()` and `cursor.All()` hand the
  bytes to the driver's own bson unmarshaler, so there is no r3 hook to run
  `Codec.Decode` on the way back into a struct field. Options, in preference
  order: (a) a bson custom registry / `ValueMarshaler` keyed on the codec'd
  fields; (b) decode into an intermediate `bson.M`/holder and post-decode the
  codec'd fields via `StructMeta.SetFieldValue`. (a) is cleaner and covers
  preloads and `Raw`; (b) is more contained but must be applied on every read
  path (Get, List, preload targets).
- **Aggregate `min`/`max` decode** - call `r3.DecodeAggregateCodecs` right before
  returning rows in `engine/mongo/aggregate.go` (it decodes codec'd group-by
  columns and `MIN`/`MAX`; `SUM`/`AVG`/`COUNT` stay raw). Same one-liner every
  backend uses.

Design background: [`plan-field-codecs.md`](./plan-field-codecs.md).

Chosen read strategy: **(b) post-decode**. Reads decode into a `bson.M`, convert
each codec'd field to its domain value, then re-marshal so the driver's native
decoder handles everything else. A per-field bson registry (option a) cannot work:
codec'd and native `time.Time` fields are indistinguishable by Go type, and the
four unix precisions are indistinguishable by stored BSON type. Codecs are keyed by
**bson field name** (built in `StructMeta.Codecs` via the new public
`r3.LookupCodec`), not the schema name, because `ParseColumnTag` - which drives
`SchemaOf` - ignores the `bson` tag. A codec'd field's bson name must therefore
match its schema column name (they agree in the flagship case). `Raw` reads are the
documented exception: the escape hatch does not apply codecs.

Checklist:
- [x] Encode on write in the `ToBSONDoc` path (and Patch/Upsert/BulkPatch values).
- [x] Wire `EncodeFilterCodecs` / `EncodeCursorCodecs` into Mongo list prep.
- [x] Decode on read (post-decode; covers Get, List, Patch/Upsert refetch, preloads).
- [x] `DecodeAggregateCodecs` before returning aggregate rows.
- [x] Remove the two `RequireCodecSupport` guards in `engine/mongo/crud.go`.
- [x] Tests: round-trip a `codec:unixtime` field; filter/cursor by a `time.Time`
      bound against the int column; `MAX` over a codec'd field decodes to a time.
- [x] `backend-parity.md`: clear the two Mongo codec rows.

## 3. Relations

Mongo has no server-side joins, so relationship resolution is emulated the same
way the GORM driver does it: a **pre-query key-set lowering**. The core types are
already backend-agnostic (`r3.RelationSpec` + builders, `r3.WithRelations`,
`Has`/`HasNo` as ordinary `FilterSpec`s, `r3.RelationAggregator`), so this is a
driver/engine change with no core churn. `engine/mongo/reflect.go` already carries
`RelationMeta` for preloads - extend that metadata, do not add a parallel one.

- **`Has` (EXISTS)** - today a relationship `FilterSpec` is silently dropped by
  `r3bson.FiltersToBSON` (returns unfiltered rows - a correctness footgun, not
  just a missing feature). Resolve by pre-querying the related collection for the
  matching foreign keys and rewriting the predicate to `_id $in <keys>` (mirror
  `drivers/gorm/relfilter.go`).
- **`HasNo` (NOT EXISTS / anti-join)** - same lowering to `$nin`; must still
  include owners whose foreign key is null.
- **`WithRelations`** - resolve explicitly-declared (table+column) relations into
  Mongo collection/field names so the two filters and the aggregate can target a
  collection the entity does not import as a Go type.
- **`AggregateThroughRelation`** (`r3.RelationAggregator`) - implement
  `AggregateThroughRelation(ctx, relation, q...)` on the Mongo BaseCRUD via
  `$lookup` + `$group` (or a key-set fold), treating `Filters` as owner filters
  and excluding soft-deleted related rows when the relation declares a
  soft-delete field. Mirror `drivers/gorm/relaggregate.go`. Note the known
  residual (R3-011): aggregating *target* columns through an M2M is out of scope
  even on GORM.

Checklist:
- [x] Make `r3bson.FilterToBSON` reject unresolved relationship filters (fail loud)
      instead of silently dropping them.
- [x] Key-set lowering for `Has` (`$in`) and `HasNo` (`$nin`, null-safe) via a
      pre-query `Distinct` on the related collection - has-many, belongs-to, m2m.
      Wired into List and Count (`engine/mongo/relfilter.go`).
- [x] Resolve `WithRelations` metadata for Mongo (`engine/mongo/relation_spec.go`;
      maps the spec's default `id` PK to Mongo's `_id`).
- [x] Implement `AggregateThroughRelation`; add `var _ r3.RelationAggregator`.
- [x] Tests: `Has`/`HasNo` over has-many, belongs-to, and m2m; null-FK anti-join;
      relation aggregation grouped and restricted by owner filter.
- [x] `backend-parity.md`: clear the four Mongo relation rows.

Note: many-to-many *preload* (loading the related structs) is still unsupported on
Mongo - out of scope here, which is about filters and aggregation. Aggregating
*target* columns through an m2m (R3-011) is out of scope on every backend.

## Suggested increments

Ship as separate PRs, easiest-value first:

1. **Upsert + BulkPatch** - self-contained, native, unblocks writes.
2. **Codecs** - biggest consumer blocker; land write + filter/cursor + read +
   aggregate together (the guard only comes off when all four are done).
3. **Relations** - `Has`/`HasNo` first (with `WithRelations`), then
   `AggregateThroughRelation`.
4. **Transactions** - next, once the above are in.

Each increment updates the Mongo column in
[`backend-parity.md`](./backend-parity.md) and, where relevant,
[`TODO.md`](./TODO.md) and [`plan-field-codecs.md`](./plan-field-codecs.md).
