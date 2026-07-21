# Plan: time-bucket GROUP BY key for aggregation

Status: **implemented** (issue #3). Core `Query.Buckets` + `Bucket`/`BucketUnit`
+ `TruncateToBucket`; lowering in the file engine, Mongo (`$dateTrunc`), and SQL
via `engine/sql.Flavor.DateTruncExpr` (raw SQL drivers, GORM, go-pg); bun
degrades loudly. Extends the `Aggregator` capability with a closed-form
date/time-bucket group key. Sibling of the time-component filter operators
(`WeekdayIn` / `TimeOfDayBetween`, see
[`plan-when-filters.md`](./plan-when-filters.md)); shares their timezone
contract and their SQL flavor seam.

Delivered vs. this plan: the group-key model is the separate `Query.Buckets`
slice (option chosen below). The SQL flavor hook landed as
`Flavor.DateTruncExpr` (a func field on `Flavor`, set per predefined flavor) so
every SQL path that carries a flavor gets buckets: the raw drivers and go-pg
directly, GORM by mapping its dialector name to a flavor. bun passes a zero
flavor (loud `ErrBucketNotSupported`) pending a dialect→flavor mapping.
Bucketing a codec'd field is rejected in validation (`ErrInvalidBucket`) pending
codec-aware bucketing. Open follow-up unchanged: the `WeekdayIn`/`TimeOfDayBetween`
SQL lowering reuses the same `Flavor` seam.

## Motivation

Time-bucketed counts are the backbone of any analytics/reporting view:

```
SELECT date(created_at) AS day, COUNT(*) AS n
FROM change_records GROUP BY date(created_at) ORDER BY day;
```

`AggregateOf` + `GroupBy` + `AggCount` is otherwise a perfect fit, but
`GroupBy("created_at")` groups by the raw instant (one group per distinct
timestamp), which is useless for a chart. The only missing piece is a
**bucketed** group key: truncate the timestamp to day / week / month before
grouping.

Design principle (inherited from when-filters): **r3 gets closed-form,
vocabulary-free primitives.** The core knows how to truncate an extracted time
component to a fixed calendar unit; it has no vocabulary ("last quarter",
"fiscal year") of its own. Any such vocabulary is a dialect concern.

## Goals

- A closed-form time-bucket group key over a time-valued field, with a fixed
  unit set: `hour` / `day` / `week` / `month` / `year`.
- The truncated value returns under a declared alias, so `AggregateRow.Time(...)`,
  `Sorts`, and `Having` all keep working by referencing that alias - exactly as
  aggregate aliases do today.
- DB-side execution on every engine: file (in-memory truncation), Mongo
  (`$dateTrunc`), and **SQL via a per-flavor hook**. Unlike when-filters, SQL is
  the flagship consumer here (the `change_records` example is GORM/SQLite), so
  the flavor hook is **in scope for the first cut**, not a deferred follow-up.

## Non-goals

- **A general "expression" group key** (a group-by entry carrying a
  dialect-rendered expression + alias). It is SQL-centric, does not port to Mongo
  / file / non-DB sources, and is the generic computed-expression framework r3
  has deliberately reserved against (`TODO(future)` in `r3_field.go`; the same
  reservation `plan-when-filters.md` makes). Only the closed, enumerated bucket
  set is in scope.
- **Arbitrary raw SQL / user-supplied expressions.** Everything is built from r3
  primitives, so it stays injection-safe and portable.
- **Per-row timezone bucketing.** See the semantics contract: buckets are cut on
  the stored wall-clock as-is, no conversion. An explicit offset argument is a
  possible additive follow-up, never the default.
- **Serialization** of bucket group keys through `dialects/json|url|yaml|toml`.
  Aggregate serialization is not wired in those dialects yet (`dialects/canonical`
  note); buckets follow whenever that lands.

## Semantics contract (the important part)

A bucket truncates the field's **stored wall-clock value, as-is**. No engine
performs timezone conversion - identical to the `WeekdayIn` / `TimeOfDayBetween`
contract:

- file engine: truncate the Go `time.Time` in its own location;
- Mongo: `$dateTrunc` with **no** `timezone` argument (BSON dates are UTC, i.e.
  exactly as stored);
- SQL: `date_trunc` / `strftime` / `DATE_FORMAT` on the stored column, no `AT
  TIME ZONE`.

Store UTC (or a normalized local wall-clock) for identical results across
backends. Apps needing per-row locality bucket a local wall-clock column.

**Week start is fixed to ISO-8601 Monday** and every engine converts at its edge
(Postgres `date_trunc('week')` is already Monday; Mongo passes
`startOfWeek: "monday"`; MySQL uses `WEEKDAY`; SQLite uses the `%w` Monday-shift
trick). Like the fixed Go weekday numbering in `WeekdayIn`, the primitive is
rigid and documented rather than configurable; a `WithWeekStart` option can be an
additive follow-up if a consumer needs it.

## Design

### 1. The group-key model: a separate `Buckets` slice (chosen)

`Query.GroupBy` is `Fields` (`[]*FieldSpec`) and `FieldSpec` is a bare
`type FieldSpec string`; every engine resolves group keys via
`FieldsToStrings(q.GroupBy)` and keys the result by the raw column name. A bucket
key carries *(source field + unit + alias)* - structurally identical to
`AggregateSpec` *(func + field + alias)*, which already lives in its **own**
`Query.Aggregates` slice beside `GroupBy`.

So a bucket goes in its own slice, mirroring that precedent:

```go
// r3_aggregate_bucket.go (core)

// BucketUnit is the closed set of calendar truncation units for a time-bucket
// group key. Week starts on ISO-8601 Monday on every backend.
type BucketUnit int8

const (
	BucketHour BucketUnit = iota + 1
	BucketDay
	BucketWeek
	BucketMonth
	BucketYear
)

func (u BucketUnit) String() string // "hour"|"day"|"week"|"month"|"year"|"unspecified"

// BucketSpec is a derived group key: the truncation of a time-valued field to
// Unit, appearing in each AggregateRow under Alias (also the name Sorts/Having
// refer to). It participates in GROUP BY alongside Query.GroupBy fields.
type BucketSpec struct {
	Field *FieldSpec
	Unit  BucketUnit
	Alias string
}

type Buckets []*BucketSpec // with Clone(), like Aggregates

// Bucket builds a bucket group key: r3.Bucket("created_at", r3.BucketDay, "day").
func Bucket(field string, unit BucketUnit, alias string) *BucketSpec
```

```go
// r3_query.go
type Query struct {
	// ...
	GroupBy    Fields
	Buckets    Buckets   // derived time-bucket group keys (participate in GROUP BY)
	Aggregates Aggregates
	Having     Filters
}
```

Usage:

```go
rows, _ := r3.AggregateOf(ctx, changeRepo, r3.Query{
	Buckets:    r3.Buckets{r3.Bucket("created_at", r3.BucketDay, "day")},
	Aggregates: r3.Aggregates{r3.AggCount("n")},
	Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("day"))},
})
for _, row := range rows {
	day, _ := row.Time("day") // truncated timestamp under the alias
	n, _ := row.Int64("n")
}
```

**Why not the alternatives:**

- *Make `FieldSpec` a struct carrying an optional transform.* Huge blast radius
  (`FieldSpec` is used by filters, sorts, fields, preloads); breaking and invasive
  for one aggregation feature. Rejected.
- *Change `GroupBy` to `[]*GroupSpec` (plain-or-bucketed entries).* Conceptually
  tidy (one list of group keys) but breaks the `GroupBy Fields` type, the
  `r3.GroupBy()` helper, and every engine's `FieldsToStrings(q.GroupBy)` call
  site. The `Aggregates`-beside-`GroupBy` precedent already establishes the
  parallel-slice idiom, and the separate slice is fully additive. Rejected in
  favor of `Buckets`, but noted as the alternative if a future unification is
  wanted.

A bucket is **not** an `AggregateFunc`: it is a group key (part of GROUP BY), not
a per-group fold. Encoding it as a bare `FieldSpec` string like `"day(created_at)"`
is the smuggle-a-transform anti-pattern the issue rejects (and it fails
`ValidateIdentifier`). The separate typed slice keeps it honest.

### 2. Query plumbing (core)

- `Query.Clone`: clone `Buckets`.
- `Query.MergeWith`: fold `Buckets` into the "aggregation shape overrides as a
  unit" block - the trigger becomes
  `len(other.GroupBy) > 0 || len(other.Buckets) > 0 || len(other.Aggregates) > 0 || len(other.Having) > 0`,
  and both branches clone `Buckets` with the rest. The shape stays all-or-nothing.
- `Query.AggregateSorts`: add each `Buckets[i].Alias` to the allowed set, so a
  grouped result may be ordered by a bucket alias.
- Validation: a bucket's `Field` goes through `ValidateIdentifier` and (where
  group fields are schema-checked) the same capability check as a plain group
  field, and must resolve to a `TypeTime` attribute. A non-time field is a typed
  error before execution - add sentinel `ErrInvalidBucket` (mirroring
  `ErrInvalidAggregate`). Empty `GroupBy` + non-empty `Buckets` is a valid grouped
  query (group solely by the bucket); empty `GroupBy` + empty `Buckets` +
  non-empty `Aggregates` stays the whole-set row.
- Shared pure helper `r3.TruncateToBucket(t time.Time, u BucketUnit) time.Time`
  (in `r3_aggregate_bucket.go`) so the file engine and any in-memory path
  truncate identically - the analog of `EvalWeekdayIn`.

### 3. Engine lowering

**file** (`engine/file/aggregate.go`): `groupAndFold` builds its composite group
key from `FieldsToStrings(q.GroupBy)`; extend it to also read each bucket's source
field (`meta.GetFieldValue`), apply `r3.TruncateToBucket`, and add the truncated
`time.Time` to the key and the output row under the alias. "Is this grouped?" now
also considers `len(q.Buckets)`.

**Mongo** (`engine/mongo/aggregate.go`): `buildGroupAndProject` builds `_id` +
`$group`; add each bucket to `_id` as
`{$dateTrunc: {date: "$created_at", unit: "day", binSize: 1, startOfWeek: "monday"}}`
(no `timezone`), and project it out under the alias. `aggregateGroupNames` grows
to carry buckets, not just `[]string`. `$dateTrunc` returns a BSON date, so
`AggregateRow.Time(alias)` gets a real `time.Time`.

**SQL** (`engine/sql/aggregate.go` + `engine/sql/flavor.go`): the bucket group
expression is flavor-specific, so it needs a hook on `Flavor`:

```go
// engine/sql/flavor.go
// DateTruncExpr renders a flavor-specific truncation of col to unit (ISO-Monday
// weeks). col is an already-quoted column reference.
DateTruncExpr(col string, unit r3.BucketUnit) (string, error)
```

| unit | Postgres | SQLite | MySQL |
|------|----------|--------|-------|
| hour | `date_trunc('hour', c)` | `strftime('%Y-%m-%d %H:00:00', c)` | `DATE_FORMAT(c,'%Y-%m-%d %H:00:00')` |
| day | `date_trunc('day', c)` | `date(c)` | `DATE(c)` |
| week | `date_trunc('week', c)` | `date(c,'-'||((strftime('%w',c)+6)%7)||' days')` | `DATE(DATE_SUB(c, INTERVAL WEEKDAY(c) DAY))` |
| month | `date_trunc('month', c)` | `strftime('%Y-%m-01', c)` | `DATE_FORMAT(c,'%Y-%m-01')` |
| year | `date_trunc('year', c)` | `strftime('%Y-01-01', c)` | `DATE_FORMAT(c,'%Y-01-01')` |

The bucket expression is aliased in the SELECT list, added to `exprByName` (so
`Having` and alias `ORDER BY` resolve), and repeated in the `GROUP BY`. Because
`dialects/sql` is flavor-neutral, the bucket rendering happens in `engine/sql`
where the `Flavor` is available (thread the flavor into
`PrepareMergedAggregateQuery`, or render bucket group exprs in `Aggregate` after
prep). SQLite/MySQL return the truncated value as TEXT/date - already covered by
`AggregateRow.Time`'s `timeLayouts` (`"2006-01-02"`, `"2006-01-02 15:04:05"`), so
no new coercion is needed.

**GORM / bun / gopg** inherit through the shared `engine/sql` aggregate prep. A
backend not wired to the flavor hook **errors loudly** (never returns un-bucketed
rows) - the same discipline as `WeekdayIn` in the SQL dialect today. Record the
per-backend state in [`backend-parity.md`](./backend-parity.md) in the same PR.

### 4. Shared flavor hook with when-filters

`DateTruncExpr` is the first member of the date/time expression family the SQL
flavor layer needs. The pending SQL lowering of `WeekdayIn` / `TimeOfDayBetween`
(`plan-when-filters.md` non-goals; a future `plan-sql-flavors.md`) wants sibling
hooks on the same `Flavor` (`WeekdayExpr`, `MinuteOfDayExpr`). Build the family's
home here and add the truncation member; when-filters' SQL lowering then adds its
members to the same seam. Sequence the two together.

### 5. Vocabulary stays in the dialects

Core gets the `BucketHour..BucketYear` enum only. Any human/URL vocabulary
(`?group_by=created_at:day`, a period keyword) belongs in `dialects/url` or a
years bridge, exactly as `WeekdayIn` lives in core while `"weekends"` lives in
`dialects/when`. Out of scope for the first cut; noted as the natural follow-up.

## Touch points, exhaustively

| file | change |
|------|--------|
| `r3_aggregate_bucket.go` (new) | `BucketUnit` + consts + `String()`; `BucketSpec`/`Buckets` + `Clone()`; `Bucket()` sugar; `TruncateToBucket()`; `ErrInvalidBucket` |
| `r3_query.go` | `Buckets` field; `Clone`, `MergeWith` (shape-override trigger + clone), `AggregateSorts` (allow bucket aliases) |
| `schema_validate.go` | validate bucket source field is a filterable/groupable `TypeTime` attribute; typed error |
| `engine/file/aggregate.go` | fold buckets into the composite group key + output row via `TruncateToBucket` |
| `engine/mongo/aggregate.go` | `$dateTrunc` in `_id`, project under alias; carry buckets through `aggregateGroupNames`/`buildGroupAndProject` |
| `engine/sql/flavor.go` | `DateTruncExpr` on the `Flavor` interface + PG/MySQL/SQLite impls |
| `engine/sql/aggregate.go` | render bucket group exprs (aliased SELECT + `exprByName` + `GROUP BY`); thread flavor |
| `drivers/gorm/*` (+ bun/gopg) | inherit via shared prep; loud error where a backend lacks the hook |
| `docs/backend-parity.md` | new row: aggregation time-bucket group key, per-backend state |
| `readme.md`, `doc.go` | document under Aggregation, with the wall-clock/UTC + ISO-Monday contract |

## Testing

- **Core:** `TruncateToBucket` table tests per unit (incl. ISO-Monday week start,
  month/year first-of-period, hour floor); `MergeWith`/`Clone`/`AggregateSorts`
  carry `Buckets`; `ErrInvalidBucket` on a non-time field.
- **file engine:** `Aggregate` grouped by a bucket alone and by `GroupBy` + bucket
  together; `Having`/`Sorts` over the bucket alias; empty-set behavior.
- **Mongo:** golden `$group`/`$dateTrunc` (day/week/month, ISO Monday, no
  timezone); end-to-end count-per-day.
- **SQL flavor:** `DateTruncExpr` goldens per flavor per unit; the SQLite/MySQL
  week trick verified against known dates; end-to-end grouped count on SQLite and
  Postgres fixtures.
- **Cross-backend equivalence:** the single most valuable test - the same
  count-per-day query over the same seed data returns the same buckets on file,
  Mongo, and SQLite (all stored UTC), pinning the semantics contract.
- **Loud degradation:** a bucket query on a not-yet-wired backend returns the
  documented unsupported error, never silent un-bucketed rows.

## Release

Additive; no breaking changes (new `Query.Buckets` field, new core symbols). Ship
core + file + Mongo + the SQL flavor hook (Postgres + SQLite at minimum, since the
flagship consumer is GORM/SQLite; MySQL in the same hook). Sequence the
when-filters SQL lowering directly behind it, reusing the flavor seam.
