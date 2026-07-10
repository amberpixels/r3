# R3 TODO / known gaps

The general engineering tracker for R3: known gaps, deferred work, and rough
edges that are **not** already owned by a more specific document. Before adding
here, check whether it belongs in one of those instead:

- **Backend feature parity** (a capability one backend has and others don't) →
  [`backend-parity.md`](./backend-parity.md).
- **A feature with an active design/rollout doc** → its `plan-*.md`
  (e.g. [`plan-field-codecs.md`](./plan-field-codecs.md),
  [`plan-upsert-and-bulk-update.md`](./plan-upsert-and-bulk-update.md)).
- **Consumer-driven feature requests** → the p44 feedback log (`R3_TODO.md` in
  the p44 repo), the authoritative source for consumer priorities.

Everything else - cross-cutting gaps, "one backend is *behind* the others",
correctness rough edges - lives here. When an item is closed, delete its row (or
move it into the relevant plan/parity doc if that becomes its home).

## Open

### Pagination

- **GORM driver has no cursor / keyset pagination.** Cursor pagination is
  implemented only in the raw SQL engine (`engine/sql.BaseCRUD`, see
  `PreparedListQuery.IsCursorPaginated` / `CursorClause` / `CursorLimit`). The
  GORM driver's `List` (`drivers/gorm/gormcrud.go`) honours only offset-based
  pagination and silently ignores `Query.Cursor`. This is the reverse of the
  usual "GORM ahead" gap. The shared query-prep already builds the cursor clause
  (and now encodes codec'd cursor keys), so wiring it into the GORM `List` is
  mostly consuming `prep.IsCursorPaginated` / `prep.CursorClause` /
  `prep.CursorLimit` / `prep.OrderBySorts()` the way `engine/sql` does.

### Field value codecs (`r3:"codec:…"`)

Full status in [`plan-field-codecs.md`](./plan-field-codecs.md); parity rows in
[`backend-parity.md`](./backend-parity.md). Remaining after the core + GORM
increments:

- **Raw `engine/sql` drivers (pq, pgx, mysql, sqlite3).** The bind path is a
  clean value pass-through; the scan path is the tricky bit - `ScanDest` hands
  live struct-field pointers to `rows.Scan`, so a codec needs a `sql.Scanner`
  wrapper (or an intermediate holder + post-scan decode), not a value map.
- **`engine/mongo`.** Write has a clean choke-point (`ToBSONDoc`); **read has no
  r3 hook** - `FindOne().Decode()` uses the driver's own bson unmarshaler, so
  reads would need a bson custom registry / `ValueMarshaler` or a reworked read
  path.
- **`engine/file`.** Single clean choke-point per direction
  (`StructMeta.GetFieldValue` / `SetFieldValue`); apply the codec at the
  serialize/deserialize boundary so in-memory filter/sort/cursor compare decoded
  domain values.
- **`drivers/bun`, `drivers/gopg`.** Prefer real per-field support; otherwise
  query-prep encode + post-scan decode. Must not silently skip.
- **Aggregate `min`/`max` on a codec'd field.** Done for GORM: the shared core
  helper `r3.DecodeAggregateCodecs` decodes codec'd group-by columns and
  `MIN`/`MAX` aggregates back to the domain value (`SUM`/`AVG`/`COUNT` stay raw).
  Each remaining backend calls it right before returning rows in the same
  increment that removes its `RequireCodecSupport` guard (a backend that can't
  store a codec can't aggregate one).
- **Dialect serialization** (`dialects/json|url|yaml|toml`) does not yet encode
  codec'd fields for REST bodies / query params.

Each backend removes its `r3.RequireCodecSupport` construction guard when wired.

## From the p44 consumer feedback log (`R3_TODO.md`)

The p44 repo's `R3_TODO.md` is the authoritative feedback log (see
[[p44-first-real-consumer]]). These are the R3-side items still open there;
copied here so they are visible from the r3 repo. Update both when one is closed.

- **R3-012 · i18n overlay on preloaded child entities.** `features/i18n` overlays
  translated fields on the decorated entity itself but **not** on entities arriving
  as preloads (e.g. `Raid.Video.Title` via `Preloads("Video")`), so a parent's
  preloaded children keep source-language text even when translations exist. Needs
  a nested-field option (e.g. `WithPreloadFields("Video", videoIDFunc, "title")`)
  that batch-fetches child translations for a page of parents and overlays them
  through the preload pointer/slice.
- **R3-011 residual · aggregate *target* columns through an M2M.**
  `AggregateThroughRelation` folds related rows grouped by a base-table column, but
  aggregating/grouping on the *target* table's columns (e.g. `MAX(activist.joined_at)`
  per squad through an M2M) is unsupported - group/aggregate columns resolve against
  the join/child table, and the target is joined only to exclude soft-deleted rows.
- **R3-006 follow-up · string-PK gorm fixture coverage.** The non-integer-PK
  Get/Delete fix is verified by p44's sqlite suite; add a string-PK case to R3's
  own Postgres gorm fixture suite for in-repo CI coverage.

## Downstream adoption waiting on a release

- **runwell** can drop its hand-rolled `unixts` GORM serializer, switch entity
  tags from `gorm:"serializer:unixts"` to `r3:"…,codec:unixtime"`, and pass
  `time.Time` bounds to `NeighborOf` (r3 now encodes filter args). Do this after
  r3 tags a release with the field-codec core + GORM bridge.
