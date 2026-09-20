# R3 TODO / known gaps

The general engineering tracker for R3: known gaps, deferred work, and rough
edges that are **not** already owned by a more specific document. Before adding
here, check whether it belongs in one of those instead:

- **Backend feature parity** (a capability one backend has and others don't) →
  [`backend-parity.md`](./backend-parity.md).
- **A feature with an active design/rollout doc** → its `plan-*.md`
  (e.g. [`plan-field-codecs.md`](./plan-field-codecs.md),
  [`plan-upsert-and-bulk-update.md`](./plan-upsert-and-bulk-update.md),
  [`plan-aggregate-buckets.md`](./plan-aggregate-buckets.md) for the time-bucket
  GROUP BY key (issue #3), [`plan-mongo-parity.md`](./plan-mongo-parity.md) for
  the Mongo catch-up work).
- **Consumer-driven feature requests** → the p44 feedback log (`R3_TODO.md` in
  the p44 repo), the authoritative source for consumer priorities.

Everything else - cross-cutting gaps, "one backend is *behind* the others",
correctness rough edges - lives here. When an item is closed, delete its row (or
move it into the relevant plan/parity doc if that becomes its home).

## Open

### Field discovery

- **`engine/sql` still drops an untagged relation-typed field silently.** A
  `[]string` or a nested struct with no `r3:"rel:"` tag is neither a column nor a
  relation, so it is skipped without a word; `r3.JSONColumn[T]` is the way to
  store one. That is correct for SQL - such a field cannot be one column - but the
  silence is what made the same behaviour cost real data on Mongo (issue #22,
  fixed by classifying relations from the declaration on the document-store
  engines). Making it loud was considered and declined: every gorm-tagged
  association slice, `internal/testing/models.go` included, would fail at
  construction. Documented in `r3utils.IsRelationType` and the readme instead.

### Field value codecs (`r3:"codec:…"`)

Full status in [`plan-field-codecs.md`](./plan-field-codecs.md); parity rows in
[`backend-parity.md`](./backend-parity.md). Remaining after the core + GORM
increments:

- **Raw `engine/sql` drivers (pq, pgx, mysql, sqlite3).** The bind path is a
  clean value pass-through; the scan path is the tricky bit - `ScanDest` hands
  live struct-field pointers to `rows.Scan`, so a codec needs a `sql.Scanner`
  wrapper (or an intermediate holder + post-scan decode), not a value map.
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

### Test suite → expectto/be

The tests are migrating from raw `t.Error`/`t.Fatal` and testify to
[`expectto/be`](https://github.com/expectto/be) (matcher-based assertions). The raw
and mixed-file passes are done; a few packages still import testify only for
out-of-scope files. Remaining r3-side follow-up:

- **Drop `betestify` (`be/x/testify`).** It is an older, separate module (rc.5,
  behind the core's rc.6) and upstream **renamed `x/testify` → `x/mock`** while
  moving the flat assertion API into core `be`. Migrate `betestify.Assert/Require`
  → `be.AssertThat/RequireThat` (or `be.Expect(t, x).To(...)`) and drop the dep.
- **Adopt the matchers that already exist**, replacing the clunky fallbacks the
  migration introduced: `be.Not(be.Nil())` → `be.NotNil()`, `be.HaveLength(0)` →
  `be.Empty()`, numeric `be.True(x >= n)` → `be_math.Gte(n)`, etc.

This migration doubled as a dogfood/stress-test of `expectto/be`. The resulting DX
and LLM-discoverability findings (what to improve in `be` itself) are written up in
the **be** repo at `docs/plan-dx-and-llm-discoverability.md`; the r3-side follow-up
above is its "Part D".

## From the p44 consumer feedback log (`R3_TODO.md`)

The p44 repo's `R3_TODO.md` is the authoritative feedback log (see
[[p44-first-real-consumer]]). These are the R3-side items still open there;
copied here so they are visible from the r3 repo. Update both when one is closed.

- **R3-011 residual · aggregate *target* columns through an M2M.**
  `AggregateThroughRelation` folds related rows grouped by a base-table column, but
  aggregating/grouping on the *target* table's columns (e.g. `MAX(activist.joined_at)`
  per squad through an M2M) is unsupported - group/aggregate columns resolve against
  the join/child table, and the target is joined only to exclude soft-deleted rows.
- **R3-006 follow-up · string-PK gorm fixture coverage.** The non-integer-PK
  Get/Delete fix is verified by p44's sqlite suite; add a string-PK case to R3's
  own Postgres gorm fixture suite for in-repo CI coverage.

## From the runwell consumer

runwell is on **mongo**, not GORM. An earlier version of this file described it as
carrying a hand-rolled `gorm:"serializer:unixts"` to be replaced by
`codec:unixtime`; that was true of an older runwell and is not true now. It has no
GORM at all, its `StartedAt` is a native BSON date needing no codec, and
`NeighborOf` already passes `time.Time` bounds. Nothing about the field-codec
rollout is downstream work for runwell. Its r3 surface is core, `dialects/when`,
`drivers/mongo` and `features/history`.

What it actually still needs the raw mongo driver for, both audited and both
currently justified:

- **Atomic increment on upsert** (`llmusage.go`, a `$inc` counter). `Upserter`
  overwrites the columns it is given; it cannot express "add to the existing
  value". This is the one that is a real r3 gap: `n = n + ?` is expressible on
  every backend r3 supports (SQL `ON CONFLICT DO UPDATE SET n = t.n + EXCLUDED.n`,
  mongo `$inc`, an in-memory add for file), so it would fit as an `UpsertOption`
  beside `OnConflict` / `UpdateOnConflict`. Not built; no issue filed yet.
- **Pipeline-expression updates** (`UpdateSlug`, a `$setDifference` over the
  document's own array field). Updating a field from an expression over other
  fields has no flavour-neutral form - array set-difference is not SQL without
  JSON functions - so the raw-driver escape hatch is the right answer and is
  documented as such on the runwell side. Not an r3 gap.
