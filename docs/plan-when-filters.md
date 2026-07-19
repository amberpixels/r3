# Plan: time-component filter operators + `when` dialect

Status: **implemented** (core operators, `dialects/when` bridge, `dialects/bson`
`$expr` lowering, file-engine evaluation, `?when=` URL support). SQL lowering
remains a follow-up (see Non-goals below). Built on years **v0.3.0**
(`schedule.WeekPatterns`).

## Motivation

Apps want URL filters like `?when=weekends` / `?when=mornings`: match rows
whose timestamp falls into a *recurring* weekly wall-clock pattern
(day-of-week set × time-of-day window). This is not expressible with the
current operator set — a recurring pattern over a raw time column is an
infinite union of ranges, so no combination of `Between`/`In` on the column
works. It is a genuinely new query capability and belongs in r3, not in app
code.

Design principle: **r3 gets vocabulary-free primitives; years owns the
words.** r3 must never know what "lunchtime" means — it only needs to compare
extracted time components. The keyword→pattern translation is a dialect
concern, exactly like `?age__gte=18` is for the Django-style parser.

## Goals

- Two orthogonal filter operators on time-valued fields: weekday membership
  and time-of-day interval — each independently useful, composing via the
  existing `And`/`Or` groups.
- A compiler from `schedule.WeekPatterns` to `r3.Filters` (the years bridge).
- First-class `?when=` support in the URL dialect for r3url-based apps.
- DB-side execution for Mongo and the in-memory file engine; a **loud,
  documented error** (never a silent drop) from the SQL dialect until
  per-flavor lowering lands.

## Non-goals (v0.1.5)

- SQL lowering. Weekday/hour extraction is flavor-specific
  (PG `EXTRACT(DOW FROM f)`, MySQL `DAYOFWEEK(f)-1`, SQLite
  `CAST(strftime('%w', f) AS INTEGER)`) and `dialects/sql` is flavor-neutral
  today. Proper support needs a flavor hook in the SQL dialect — a separate,
  self-contained follow-up (`plan-sql-flavors.md` when we get there). The
  operators are designed so that follow-up is additive.
- A generic "computed expression" filter framework. `Attribute.Computed` stays
  reserved; these two operators are deliberately narrow and closed-form.

## Semantics contract (the important part)

Both operators evaluate the **stored wall-clock value of the field, as-is**.
No engine performs timezone conversion:

- file engine: `t.Weekday()` / `t.Hour()*60 + t.Minute()` on the Go value;
- Mongo: `$dayOfWeek` / `$hour` / `$minute` with **no** `timezone` argument
  (BSON dates are interpreted as UTC — i.e. exactly as stored).

Apps that need locality (per-row timezones, e.g. runwell's runs across
countries) store a *local wall-clock* column and filter on it. This contract
is what keeps the feature backend-portable and r3 timezone-free; document it
on the operators and in the README.

## Design

### 1. Root operators (`r3` package)

```go
// r3_filter_operator.go
OperatorWeekdayIn        // weekday-of(field) ∈ value
OperatorTimeOfDayBetween // minute-of-day-of(field) ∈ [lo, hi); lo > hi wraps midnight
```

```go
// r3_filter.go sugar
// WeekdayIn matches rows whose time field's weekday is one of days.
func WeekdayIn(field string, days ...time.Weekday) *FilterSpec

// TimeOfDayBetween matches rows whose time field's minute-of-day lies in
// [loMin, hiMin). loMin > hiMin means the interval wraps midnight
// (e.g. 1320, 300 = 22:00–05:00).
func TimeOfDayBetween(field string, loMin, hiMin int) *FilterSpec
```

Value encoding (wire-friendly, no new codec machinery):

- `WeekdayIn`: `[]time.Weekday` from the sugar; dialects/engines normalize via
  a shared helper accepting `[]time.Weekday`, `[]int`, `[]float64` (JSON
  decode) → `[]time.Weekday`. Canonical numbering is **Go's**: 0=Sunday …
  6=Saturday. Every lowering converts to its backend's numbering at the edge
  (Mongo `$dayOfWeek` is 1=Sunday…7).
- `TimeOfDayBetween`: `[]any{loMin, hiMin}` — same shape as `Between`, so
  `ExtractBetweenBounds` is reused for validation. Minutes-of-day
  (`0..1439`); minute precision matches `schedule.TimeOfDay` and avoids
  seconds edge-cases.

The enum is closed and guarded by the `exhaustive` linter — adding the two
constants makes the compiler enumerate every switch that needs a decision
(`String()`, canonical map, bson, sql, file engine, schema caps). That is the
extension mechanism working as designed; no registration API is warranted for
two operators.

### 2. Touch points, exhaustively

| file | change |
|------|--------|
| `r3_filter_operator.go` | 2 consts + `String()` cases (`"weekday_in"`, `"tod_between"`) |
| `r3_filter.go` | `WeekdayIn`, `TimeOfDayBetween` sugar |
| `dialects/canonical/operators.go` | `"weekday_in"`, `"tod_between"` entries (reverse map is derived) |
| `schema_caps.go` | add both to `TypeTime` `defaultOps` so schema advertisement includes them |
| `engine/file/filter.go` | 2 cases in `evaluateOperator`: `toTime(fieldVal)` (non-time field value → error, mirroring existing style), weekday membership; minute-of-day with wrap handling |
| `dialects/bson/dialect.go` | lower both in `FilterToBSON` via `$expr` (see below); `OperatorToBSON` returns the "compound, handled in FilterToBSON" error like Between |
| `dialects/sql/dialect.go` | explicit `unsupported operator %s for SQL dialect (see plan-when-filters.md)` error — loud, not skipped |
| `dialects/json` (and yaml/toml if they alias operators) | ensure the canonical names round-trip; values are plain ints, nothing special |
| `schema_validate.go` | no change (validates `Filterable`, not ops) — but add a test asserting the new ops pass validation on a filterable time attribute |

Mongo lowering (`$expr` — component extraction has no plain-query form):

```js
// WeekdayIn (Go weekdays converted to Mongo's 1..7 at lowering):
{$expr: {$in: [{$dayOfWeek: "$started_at_local"}, [1, 7]]}}

// TimeOfDayBetween, minuteOfDay := {$add: [{$multiply: [{$hour: "$f"}, 60]}, {$minute: "$f"}]}
// non-wrapping [lo, hi):
{$expr: {$and: [{$gte: [minuteOfDay, lo]}, {$lt: [minuteOfDay, hi]}]}}
// wrapping (lo > hi):
{$expr: {$or:  [{$gte: [minuteOfDay, lo]}, {$lt: [minuteOfDay, hi]}]}}
```

`$expr` documents compose fine as members of the existing `$and`/`$or`
groups. **Perf note (document, don't solve):** `$expr` component extraction
cannot use an index — fine for the collection sizes r3's Mongo driver serves
today. Apps at index-hungry scale denormalize a weekday/minute column and use
plain `In`/`Between`; the primitives don't preclude that, and the `when`
dialect below could later accept a column-mapping strategy.

### 3. `dialects/when` — the years bridge (new package `r3when`)

Why a dialect: dialects translate external representations into r3 specs.
`"weekends"` is an external (human) representation of a filter — same category
as a Django param or a JSON filter document. Putting the bridge here keeps the
root package free of the years dependency (today only `features/*` import
years; root stays lean).

```go
package r3when // dialects/when

// Filters compiles week patterns into r3 filters against field.
// Group = AND(weekday, OR(spans)); list = OR of groups.
func Filters(field string, ps schedule.WeekPatterns) r3.Filters

// Parse resolves a vocabulary expression ("weekends", "mornings,evenings")
// via years and compiles it. The years error (which lists known terms) is
// passed through — it is user-facing.
func Parse(field, expr string) (r3.Filters, error)

// ParseWith uses a custom vocabulary (custom partition / app terms).
func ParseWith(v schedule.Vocabulary, field, expr string) (r3.Filters, error)
```

Compilation of one `WeekPattern{Days, Spans}` — correct from day one for every
shape the type allows, including combinations the v1 vocabulary never emits:

1. Empty `Days` and empty `Spans` → no filter (matches everything).
2. Spans within one day (`End.Hour < 24`): span → `TimeOfDayBetween(lo, hi)`;
   result `And(WeekdayIn(days...), Or(spanFilters...))`, dropping whichever
   side is empty and unwrapping single-element groups.
3. Cross-midnight span (`End.Hour >= 24`) **with** a day constraint — the
   attribution rule (years plan: "sunday night" includes Monday 01:00) lowers
   to a disjunction with shifted weekdays:
   `Or(And(WeekdayIn(days), TimeOfDayBetween(lo, 1440)), And(WeekdayIn(days+1), TimeOfDayBetween(0, hi-1440)))`.
4. Cross-midnight span with **no** day constraint degenerates to the single
   wrapping `TimeOfDayBetween(lo, hi mod 1440)` — cheaper and equivalent.

`WeekPatterns` → `Or` across compiled groups (single group unwrapped). The
output uses only root filter vocabulary, so it serializes through every
existing dialect and runs on any engine that lowers the two operators.

Dependency change: `go.mod` bumps `amberpixels/years` to v0.3.0; it becomes a
*direct* dependency of `dialects/when` (and `dialects/url`) only.

### 4. URL dialect: `?when=`

- `ParamNames.When` (default `"when"`), included in `reservedParamNames()` so
  Django-style parsing never eats it.
- `FilterConfig`: `AllowWhen bool`, `WhenField string`.
- Option: `func WithWhenFilter(field string) Option` — enables and pins the
  target column. The field goes through `r3.ValidateIdentifier` like every
  other field source.
- `parse_decomposed.go`: when enabled and the param is present, call
  `r3when.Parse(cfg.Filter.WhenField, value)` and append to parsed filters.
  **Unknown keyword is an error**, not a skip — unlike Django params (where an
  unknown suffix plausibly isn't a filter), `?when=` only exists because the
  app opted in, so garbage is a client error and the years message (listing
  valid terms) is exactly what belongs in the 400.
- Formatting: one-way. Compiled filters don't round-trip to the keyword;
  `FormatDecomposed` emits them as ordinary filter JSON. Document the
  asymmetry (same status as `Has`/`HasNo` which "do not round-trip").

## Testing

- File engine: table tests per operator (weekday hit/miss, tod boundaries
  lo/hi inclusive-exclusive, wraparound, non-time field → error, `*time.Time`
  via `toTime`), plus end-to-end `List` with composed `And`/`Or`.
- BSON dialect: golden `bson.D` for both operators, wrap and non-wrap,
  weekday numbering conversion, composition inside `And`/`Or`.
- `r3when`: compilation goldens for shapes 1–4 above; `Parse` happy path and
  error passthrough. Property-style cross-check: for a grid of times across
  two weeks, `pattern.Contains(t)` must equal file-engine evaluation of the
  compiled filters on a record with that timestamp — **this test pins the
  years/r3 semantic equivalence** and is the single most valuable test in the
  plan.
- URL dialect: `?when=` parse, unknown keyword error, reserved-name
  non-collision with Django mode, disabled-by-default behavior.
- SQL dialect: asserts the explicit unsupported error.
- Canonical/JSON: operator name round-trip.

## Docs

README: add both operators to "Available operators" with the wall-clock
contract; extend the URL Query Parsing section with `WithWhenFilter`; note
Mongo `$expr`/index caveat and SQL non-support.

## Release

Tag **v0.1.5** after years v0.3.0 is tagged. Additive only; no breaking
changes. Follow-ups queued behind it: SQL flavor hook + lowering; optional
`when` formatting round-trip via retaining the source expression.
