# Plan: Decode `min`/`max` aggregate results for codec'd fields

Status: **Core + GORM landed.** The shared helper `r3.DecodeAggregateCodecs` is
implemented and wired into the GORM `Aggregate` path (the only backend whose
`RequireCodecSupport` guard is lifted today). Remaining backends
(raw SQL / bun / gopg / mongo / file) call the same helper in the increment that
removes their guard — see §2. Follow-on to the field-value-codec rollout
([`plan-field-codecs.md`](./plan-field-codecs.md)). Authored from the **p44**
consumer side (the audit/stats reads that motivated `Aggregator` in R3-010).

## The gap

A value codec (`r3:"...,codec:unixtime"`) bridges an attribute's **domain** type
(`time.Time`) to its **stored** type (`int` unix seconds) on read, write, and
filter/cursor args. It does **not** yet cover **aggregate result values**.

Concretely: with `StartedAt time.Time \`r3:"started_at,codec:unixtime"\``, the
column is physically `INTEGER`, so

```go
rows, _ := r3.AggregateOf(ctx, repo, r3.Query{
    GroupBy:    r3.GroupBy("location_id"),
    Aggregates: r3.Aggregates{r3.AggMax("started_at", "last_started")},
})
rows[0].Time("last_started")   // (zero, false) — WRONG
rows[0].Int64("last_started")  // (1752000000, true) — the raw stored unix int
```

`MAX(started_at)` over an integer column returns an **int64** in the
`AggregateRow`. `AggregateRow.Time` only knows how to parse `time.Time` and the
textual SQLite datetime layouts (`timeLayouts` in `r3_aggregate.go`) — it has no
idea this int64 is a unix stamp, so it fails. The codec that *could* interpret it
is never consulted on the aggregate read path.

This is the one aggregate rough edge called out in
[`backend-parity.md`](./backend-parity.md):

> | Value codecs — aggregate `min`/`max` decode on a codec'd field | ❌ returns the raw stored int | ❌ | ❌ | ❌ | ❌ |

Note this is the mirror image of the pre-codec world: `AggregateRow.Time`'s
`timeLayouts` fallback exists precisely because SQLite `MIN/MAX` over a **plain**
datetime (TEXT) column returns text. Once a field is codec'd to `int`, that
textual path no longer fires — a numeric path is needed instead, and only the
codec knows the precision.

## Goal / non-goals

**Goal:** for a codec'd attribute, an aggregate result value that still carries
the attribute's **domain meaning** is decoded back to the domain value before it
lands in the `AggregateRow`, so `row.Time("last_started")` returns the real
instant — uniformly across every backend that implements `Aggregator`.

**Which result columns get decoded** — this is the crux of the design:

| Result column                       | Decode? | Why |
|-------------------------------------|:-------:|-----|
| **Group-by** column that is codec'd | ✅ | The grouped value **is** the field's domain value (e.g. `GROUP BY started_at`). |
| `MIN(field)` / `MAX(field)`         | ✅ | Min/max preserve the domain type — the extremum is a real field value. |
| `SUM(field)`                        | ❌ | A sum of unix seconds is **not** a `time.Time`; decoding would be nonsense. |
| `AVG(field)`                        | ❌ | Same — not a domain value. |
| `COUNT` / `COUNT(DISTINCT field)`   | ❌ | Always an integer count, never the domain type. |

So decode fires for **(a)** codec'd group-by columns and **(b)** `AggregateMin` /
`AggregateMax` aggregates whose `Field` is codec'd. Everything else passes
through untouched.

**Non-goals:**
- Aggregating a codec'd **target** column *through a relation*
  (`AggregateThroughRelation`) — that's the separate R3-011 residual gap (M2M
  target aggregation is unsupported regardless of codecs). Out of scope; see
  Follow-ups.
- Any new public API. `AggregateRow` and its accessors are unchanged; this is a
  correctness fix in how rows are populated.
- DDL / physical column types — unchanged (r3 never owns DDL).

## Design

### 1. Shared core helper (`r3_codec.go` or `r3_aggregate.go`)

Add a backend-neutral, in-place decoder mirroring the existing
`EncodeFilterCodecs` / `EncodeCursorCodecs` shape:

```go
// DecodeAggregateCodecs decodes, in place, every AggregateRow value that
// carries a codec'd attribute's domain meaning back to the domain value:
//   - a group-by column that maps to a codec'd attribute, and
//   - a MIN/MAX aggregate over a codec'd attribute (the extremum is a real
//     field value; SUM/AVG/COUNT are NOT decoded — they are not domain values).
// It is a no-op when the schema declares no codecs. Backends call it just
// before returning aggregate rows so decode happens once, uniformly.
func DecodeAggregateCodecs(s Schema, q Query, rows []AggregateRow) error
```

Implementation sketch:

```go
func DecodeAggregateCodecs(s Schema, q Query, rows []AggregateRow) error {
    if !s.hasCodecs() || len(rows) == 0 {
        return nil
    }
    // result key -> codec to apply
    decoders := map[string]Codec{}
    for _, g := range q.GroupBy {
        if attr, ok := s.Lookup(g.String()); ok && attr.Codec != nil {
            decoders[g.String()] = attr.Codec
        }
    }
    for _, a := range q.Aggregates {
        if a == nil || a.Field == nil {
            continue
        }
        if a.Func != AggregateMin && a.Func != AggregateMax {
            continue // SUM/AVG/COUNT/COUNT_DISTINCT never preserve the domain type
        }
        if attr, ok := s.Lookup(a.Field.String()); ok && attr.Codec != nil {
            decoders[a.Alias] = attr.Codec
        }
    }
    if len(decoders) == 0 {
        return nil
    }
    for _, row := range rows {
        for key, c := range decoders {
            v, ok := row[key]
            if !ok || v == nil { // absent or SQL NULL (e.g. MAX over an empty group)
                continue
            }
            decoded, err := c.Decode(v, nil) // nil target -> codec's natural domain type
            if err != nil {
                return err
            }
            row[key] = decoded
        }
    }
    return nil
}
```

**`target = nil` is deliberate.** `Attribute` carries only the logical
`DataType` + `Codec`; it does **not** retain the Go `reflect.Type`. An
`AggregateRow` is a `map[string]any` with no destination struct field, so there
is no pointer/nullable shape to honor. The `Codec.Decode` contract already
handles a nil target as "the natural domain value": `unixTimeCodec.Decode` ->
`zeroForTarget(nil)` / `wrapForTarget(t, nil)` -> a bare `time.Time`. That is
exactly what `row.Time(key)` then returns via its `case time.Time` branch. Any
custom codec must likewise treat a nil target as its natural type (note this in
the `Codec.Decode` doc comment as part of this change).

**NULL groups:** `MAX` over an empty/all-NULL group yields SQL `NULL` -> `nil` in
the row; the `v == nil` guard leaves it as `nil` (not the codec's zero-time), so
`row.Time` still reports `ok=false` for a genuinely absent extremum. Keep it that
way — decoding `nil` to the zero time would fabricate `1970-01-01`.

### 2. Wire every `Aggregator` backend

Each engine calls the helper on the fully-merged query right before returning.
The merged query is already in scope at every call site.

| Backend | File · function | Merged query in scope as | Call site |
|---|---|---|---|
| raw SQL engine (pq/pgx/mysql/sqlite3) | `engine/sql/aggregate.go` · `BaseCRUD.Aggregate` | `prep.Query` (embedded `PreparedListQuery.Query`) | after `ScanAggregateRows(rows)` (line ~205) |
| GORM driver | `drivers/gorm/gormcrud.go` · `GormCRUD.Aggregate` | `prep.Query` | after `enginesql.ScanAggregateRows(rows)` (line ~254) |
| mongo | `engine/mongo/aggregate.go` · `BaseCRUD.Aggregate` | merged `q` | after the `rows` loop (line ~92) |
| file | `engine/file/aggregate.go` · `BaseCRUD.Aggregate` | merged `q` | after `groupAndFold` / before return (line ~61–74) |

Pattern (SQL/GORM example):

```go
rows, err := enginesql.ScanAggregateRows(rows)
if err != nil {
    return nil, err
}
if err := r3.DecodeAggregateCodecs(r.Schema, prep.Query, rows); err != nil {
    return nil, err
}
return rows, nil
```

Notes per backend:
- **GORM & raw SQL:** the codec'd column is physically `INTEGER`, so `MIN/MAX`
  return an `int64` — `unixTimeCodec.Decode`'s `asInt64` path handles it (and its
  string/`[]byte` fallbacks cover any driver that stringifies). Once wired, the
  raw-SQL codec guard removal is tracked separately in `plan-field-codecs.md`;
  aggregate decode should land in the **same** increment that removes each
  backend's `RequireCodecSupport` guard (a backend that can't store a codec can't
  aggregate one either).
- **file engine:** it holds decoded domain values in memory already (the codec is
  applied at its serialize boundary per the codec plan). Verify whether `MIN/MAX`
  there yield a `time.Time` (already fine, helper is a no-op for it) or the stored
  int (needs the helper). Wire the call regardless — the helper is idempotent-safe
  only if values are still stored form, so if the file engine already returns
  `time.Time`, `Decode(time.Time, nil)` must round-trip it (the unix codec's
  `asTime` path does). Confirm and add a test either way.
- **mongo:** blocked behind the codec read-seam work in `plan-field-codecs.md`
  (mongo has no r3 read hook today). Add the `DecodeAggregateCodecs` call when
  mongo codecs land; until then mongo's construction guard still fires, so this is
  a no-op there.

Because it is one shared core function, "which columns decode" logic lives in
exactly one place and cannot drift between backends.

### 3. Tests

Core unit test (`r3_codec_test.go` or `r3_aggregate_test.go`), no backend:
- `DecodeAggregateCodecs` decodes a `MAX` alias and a codec'd group-by column to
  `time.Time`; leaves a `SUM`/`AVG`/`COUNT` alias as the raw int; leaves
  non-codec'd columns untouched; is a no-op on a zero schema; leaves a `nil`
  (empty-group `MAX`) as `nil`.

Backend round-trip (extend the existing gorm-over-sqlite codec suite — no Docker,
mirrors how the codec core was verified):
- Seed rows with codec'd `time.Time` values across a couple of group keys.
- `AggMax`/`AggMin("started_at", ...)` grouped by a plain column ->
  `row.Time(alias)` returns the true max/min instant (assert equality to the
  seeded time, UTC).
- `AggCount` in the same query still returns the integer count (regression: proves
  count is not decoded).
- A group with no rows / all-NULL codec'd values -> `row.Time` reports `ok=false`.
- Optionally: `GROUP BY started_at` -> the group-key column decodes to `time.Time`.

### 4. Docs to update in the same PR

- **`docs/backend-parity.md`:** flip the "aggregate `min`/`max` decode" row from
  `❌` to `✅` for each backend as it lands (delete the row once all supported
  backends are ✅).
- **`docs/plan-field-codecs.md`:** note aggregate-decode as landed in the
  status/`min`/`max` follow-up bullet (§ "Later" / the aggregate caveat).
- **`docs/TODO.md`:** remove the "Aggregate `min`/`max` on a codec'd field
  returns the raw stored int" bullet under *Field value codecs* once done.
- **p44 `R3_TODO.md`:** this closes the aggregate caveat noted under **R3-010**;
  add a one-line "aggregate min/max on codec'd fields now decodes (r3 vX.Y.Z)"
  and, when p44 actually adopts a codec'd timestamp, drop the note. (p44 does not
  use codecs today — see below.)

## Consumer context (why this matters, and its current urgency)

p44 is the consumer that drove `Aggregator` (R3-010: `svcraid.StatsByLocation`,
`svcsquadstats` last-raid dates), and those reads use `row.Time` over `MAX`
datetime columns — the exact path this touches. **Today p44 stores timestamps as
SQLite datetime TEXT, not codec'd ints**, so it currently relies on
`AggregateRow.Time`'s textual `timeLayouts` fallback and is unaffected. This fix
is what makes it **safe for p44 (or runwell) to migrate a hot timestamp column to
`codec:unixtime`** without silently breaking its stats/audit aggregates — i.e. it
removes the last correctness footgun blocking codec adoption on aggregated
columns. Priority is "before any consumer codec's an aggregated timestamp," not
"urgent for a shipping feature."

## Follow-ups (explicitly out of scope)

- **Relation aggregation** (`AggregateThroughRelation`,
  `drivers/gorm/relaggregate.go`): if/when target-column aggregation through an
  M2M is supported (R3-011 residual), it will need the same decode pass keyed off
  the target schema's codecs. Note it there; do not build it here.
- **`SUM`/`AVG` semantics for codec'd fields:** intentionally undecoded. If a real
  use case ever wants "sum of durations" as a domain type, that's a separate
  design (a codec whose stored+summed form is itself meaningful), not this fix.

## References

- `r3_codec.go` — `Codec`, `unixTimeCodec`, `EncodeFilterCodecs` /
  `EncodeCursorCodecs` (the sibling encoders this mirrors), `hasCodecs`,
  `zeroForTarget` / `wrapForTarget` (nil-target handling).
- `r3_aggregate.go` — `AggregateRow`, `.Time` / `.Int64` accessors, `timeLayouts`,
  `AggregateSpec` (`Func`, `Field`, `Alias`), `Aggregator`, `AggregateOf`.
- `engine/sql/aggregate.go`, `drivers/gorm/gormcrud.go`,
  `engine/mongo/aggregate.go`, `engine/file/aggregate.go` — the four `Aggregate`
  implementations to wire.
- `schema.go` — `Attribute` (carries `Codec`; no `reflect.Type`, hence
  `target=nil`).
- `docs/plan-field-codecs.md`, `docs/backend-parity.md`, `docs/TODO.md`,
  p44 `R3_TODO.md` (R3-010).
