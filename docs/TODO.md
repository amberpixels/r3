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

Everything else — cross-cutting gaps, "one backend is *behind* the others",
correctness rough edges — lives here. When an item is closed, delete its row (or
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
  clean value pass-through; the scan path is the tricky bit — `ScanDest` hands
  live struct-field pointers to `rows.Scan`, so a codec needs a `sql.Scanner`
  wrapper (or an intermediate holder + post-scan decode), not a value map.
- **`engine/mongo`.** Write has a clean choke-point (`ToBSONDoc`); **read has no
  r3 hook** — `FindOne().Decode()` uses the driver's own bson unmarshaler, so
  reads would need a bson custom registry / `ValueMarshaler` or a reworked read
  path.
- **`engine/file`.** Single clean choke-point per direction
  (`StructMeta.GetFieldValue` / `SetFieldValue`); apply the codec at the
  serialize/deserialize boundary so in-memory filter/sort/cursor compare decoded
  domain values.
- **`drivers/bun`, `drivers/gopg`.** Prefer real per-field support; otherwise
  query-prep encode + post-scan decode. Must not silently skip.
- **Aggregate `min`/`max` on a codec'd field** returns the raw stored int rather
  than a decoded domain value (all backends, GORM included). Decode aggregate
  result values for codec'd group/aggregate columns in `engine/*/aggregate.go`.
- **Dialect serialization** (`dialects/json|url|yaml|toml`) does not yet encode
  codec'd fields for REST bodies / query params.

Each backend removes its `r3.RequireCodecSupport` construction guard when wired.

## Downstream adoption waiting on a release

- **runwell** can drop its hand-rolled `unixts` GORM serializer, switch entity
  tags from `gorm:"serializer:unixts"` to `r3:"…,codec:unixtime"`, and pass
  `time.Time` bounds to `NeighborOf` (r3 now encodes filter args). Do this after
  r3 tags a release with the field-codec core + GORM bridge.
