# R3 Schema — Design & Implementation Plan

> Status: **proposed**, not yet implemented.
> Audience: the engineer (human or agent) implementing this in the `r3` repo.
> Scope of this document: **R3 only**. The consuming app (p44: URL↔filter sync,
> `/-/schema` HTTP endpoint, dynamic admin tables) is explicitly out of scope and
> tracked separately. This plan must land as self-contained R3 changes.

---

## 1. Motivation

R3 today has a half-built notion of "what an entity's fields are": `engine/sql.StructMeta`
(`engine/sql/reflect.go`), derived from the `r3`/`db` struct tags via `internal/tag`.
It knows column names, the PK, the soft-delete column, and relations, and it encodes
exactly **one** capability — `ValidatePatchColumns` rejects writes to the PK and the
soft-delete column.

What's missing, and what this plan adds:

- **No filter/sort/field validation on reads.** The list path
  (`drivers/*/...crud.go` → `engine/sql.PrepareMergedListQuery`) feeds request-supplied
  filter/sort/select fields straight into SQL. An unknown field name reaches the DB and
  surfaces as a driver error (HTTP 500 in consumers). There is no typed
  "this field isn't filterable" error.
- **No per-field capability model.** Nothing expresses *filterable / sortable / queryable
  (selectable) / creatable / mutable* per field.
- **No introspection.** Nothing a consumer can serialize to describe an entity to a
  frontend (column pickers, a dynamic `+ filter` UI).
- **No room for computed fields.** `features/metrics` is operational CRUD telemetry, not
  query-time aggregation. A field like `raids_created` (a live `COUNT`) cannot be modeled.
- **A latent write bug.** `engine/sql` Update writes all non-PK columns
  (`StructMeta.NonPKFieldValues` excludes only the PK), so a full `Update` can clobber
  `created_at` or resurrect a soft-deleted row. Modeling mutability fixes this.

This plan promotes the schema to a **first-class, capability-bearing concept in the core
`r3` package**, derived from tags with sane zero-config defaults, consumed by the engines
for validation and write-shaping, and serializable for introspection.

---

## 2. Concepts & vocabulary (decided)

These were settled in design discussion; treat them as fixed unless a blocker appears.

### 2.1 `Schema` and `Attribute`

- **`r3.Schema`** — the logical descriptor of an entity: an ordered set of `Attribute`s
  plus lookup helpers. **Logical, engine-agnostic, zero-dependency** — it lives in the
  core `r3` package next to `Query`/`Filters`. It contains **no SQL column names**.
- **`r3.Attribute`** (`Attr` for short in prose) — one declared, capability-bearing member
  of an entity.

Naming rationale: `r3.Fields`/`r3.FieldSpec` are **already taken** — a `FieldSpec` is a
*reference to a queryable thing inside a Query*. So:

> **Attribute** = a declared member of an entity (the schema).
> **Field/FieldSpec** = a reference to one inside a query.
> **The Schema is the set of valid Fields, plus what each is allowed to do.**

Do **not** name schema members "Field" or "Column" (the latter wrongly implies a literal
DB column, which computed attributes are not).

### 2.2 Capabilities

Five capabilities, modeled as a bitset on each attribute:

| Capability   | Meaning                                                        |
|--------------|----------------------------------------------------------------|
| `Filterable` | may appear in `Query.Filters`                                  |
| `Sortable`   | may appear in `Query.Sorts`                                    |
| `Queryable`  | may appear in `Query.Fields` (SELECT) and in serialized output |
| `Creatable`  | may be set by Create                                          |
| `Mutable`    | may be changed after creation — gates **both** Update and Patch |

**Write model = two orthogonal axes (`Creatable`, `Mutable`), not three.** There is no
"updatable but not patchable" axis: Patch is a strict subset of Update, and a field
forbidden to Patch but allowed via full Update would just be a silent backdoor. The four
combinations cover the real cases:

| Creatable | Mutable | Meaning                | Example                          |
|-----------|---------|------------------------|----------------------------------|
| ✓ | ✓ | freely editable        | `title`, `status`                |
| ✓ | ✗ | **set-once / immutable** | `slug`, `created_by`           |
| ✗ | ✗ | server/computed read-only | `id`, `created_at`, computed  |
| ✗ | ✓ | (rare, allowed for free) | a field only set post-create   |

`Creatable && !Mutable` is exactly "immutable after creation".

### 2.3 Capabilities are the *public* contract — permissions narrow, never widen

`Mutable=false` means **immutable to every API caller, superadmin included** — used for a
field whose source of truth is external (e.g. a nightly third-party feed). It is **not**
"this user lacks permission":

- **Schema caps are the ceiling** of what any user may do.
- **`features/permissions` only narrows** that ceiling per-actor/row. Nothing in
  permissions can grant write access to a `Mutable=false` attribute.

Role-shaped rules ("admin may set `featured`, a user may not") belong entirely to
`permissions` — they must **not** leak into the schema. A field an admin can edit is a
*mutable* field; the user merely lacks permission.

### 2.4 The structural floor (engine, absolute)

Independent of capabilities, the engine enforces a physical floor that **no one** can
cross, not even system code:

- A **computed** attribute (no backing column) can never be written — there is nowhere to
  put the value.
- The **PK** is identity, not a writable field.

This floor is intrinsic (it falls out of "is there a real column?"), so no escape hatch,
however raw, can corrupt identity or computed values. The worst any bypass can do is write
a *real* column the API contract would have refused.

### 2.5 System / worker writes = a separate door, not a privilege tier

A worker that owns a feed-synced field must be able to write it even though it is
`Mutable=false` for all users. This is handled by an **explicit, per-call bypass** of the
write-capability enforcement (see §6), **not** by inspecting actor claims and **not** by a
"system > admin > user" hierarchy inside the schema. Worker writes still pass through
`history`/`metrics` decorators (they remain audited); they only skip the capability check.

### 2.6 Cross-engine

R3's thesis ("features compose across backends") requires the schema to be engine-neutral:

- **`r3.Schema` is logical**: attribute names, types, caps, allowed operators, enum values,
  relation refs, computed marker. It is what dialects, validation, and (later) a frontend
  see.
- **Each engine owns the physical binding**, keyed by **attribute name**: `engine/sql`
  maps name → column (today's `StructMeta`); a Mongo engine would map name → BSON path.
  Filters/sorts reference attribute *names*; the engine translates name → column/path, so
  the same `r3.Query` validates and runs against PG, MySQL, or Mongo.
- The only genuinely engine-specific piece is a computed attribute's expression (SQL
  `COUNT(...)` vs Mongo `$group`). Model `Compute` as portable primitives
  (`Count(relation)`, `Sum`, `Avg`) plus a SQL-only raw-expression escape hatch.
  **Computed execution is out of scope here** — see §8.

### 2.7 Defaults (permissive opt-out)

Derived defaults for a plain scalar column with no extra tags:

| Capability | Default | Exceptions |
|------------|---------|------------|
| Queryable  | ✓ | — |
| Filterable | ✓ | scalars only (int/string/bool/time/enum) |
| Sortable   | ✓ | scalars only |
| Creatable  | ✓ | **except** PK, `created_at`/`updated_at`, soft-delete |
| Mutable    | ✓ | **except** PK, `created_at`, soft-delete (`updated_at` is system-only) |

- **Relations**: Queryable (preload) yes; Filterable via the relation mechanism only;
  Sortable no.
- **JSON/blob/text columns**: Queryable yes; Sortable no by default.
- Rationale for opt-out: p44 (and any current R3 consumer) has **no** validation today, so
  permissive defaults are a strict Pareto improvement — same capabilities, now *validated*
  (typed error instead of a SQL 500) and *introspectable*. Tags only tighten.
- Provide a `DefaultsStrict()` policy (opt-in caps, allowlist posture) for consumers who
  want lock-down by default. Default policy stays permissive.

---

## 3. Public API surface (target)

New file `schema.go` (+ `schema_caps.go`, `schema_derive.go`, `schema_validate.go` as
convenient) in the **core `r3` package**. Sketch — finalize signatures during
implementation, keep them minimal and immutable-friendly like the rest of the query model:

```go
// DataType is the logical type of an attribute, used to pick default operators
// and (later) to drive a frontend's filter widgets.
type DataType string

const (
    TypeInt    DataType = "int"
    TypeFloat  DataType = "float"
    TypeString DataType = "string"
    TypeBool   DataType = "bool"
    TypeTime   DataType = "time"
    TypeEnum   DataType = "enum"
    TypeJSON   DataType = "json"
    TypeRel    DataType = "relation"
)

type Capability uint8

const (
    Filterable Capability = 1 << iota
    Sortable
    Queryable
    Creatable
    Mutable
)

type Attribute struct {
    Name     string                 // public/wire name, e.g. "created_at" (snake_case)
    Type     DataType
    Caps     Capability
    Ops      []FilterOperatorSpec   // allowed filter operators; nil ⇒ defaults per Type
    Enum     []string               // for Type==TypeEnum
    Relation *RelationRef           // for Type==TypeRel (target entity, label attr)
    Computed bool                   // reserved; true ⇒ no backing column (see §8)
}

func (a Attribute) Has(c Capability) bool { return a.Caps&c == c }

type Schema struct {
    // name index + ordered slice; unexported fields
}

func (s Schema) Attributes() []Attribute
func (s Schema) Lookup(name string) (Attribute, bool)
func (s Schema) Filterable(name string) bool
func (s Schema) Sortable(name string) bool
func (s Schema) Queryable(name string) bool
func (s Schema) Writable(name string, op WriteOp) bool   // WriteOp = Create | Mutate

// ValidateQuery is THE source of typed 400-class errors. It checks every
// referenced field in Filters, Sorts, and Fields against the schema's caps.
func (s Schema) ValidateQuery(q Query) error
```

Derivation + override builder:

```go
// SchemaOf reflects T's tags into a logical Schema with default capabilities.
// Cached per type T (see §5.5).
func SchemaOf[T any](opts ...SchemaOption) Schema

// Builder overrides for what tags can't express (computed attrs, op restrictions,
// relation labels, strict defaults). Returns a new Schema (immutable).
func (s Schema) With(attrs ...Attribute) Schema
```

### 3.1 Typed errors (new, in `r3_errors.go`)

```go
var ErrFieldNotFilterable = errors.New("field is not filterable")
var ErrFieldNotSortable   = errors.New("field is not sortable")
var ErrFieldNotQueryable  = errors.New("field is not queryable")
var ErrUnknownField       = errors.New("unknown field")
// reuse existing ErrInvalidPatchField / ErrNoPatchFields for writes;
// generalize ErrInvalidPatchField wording to cover Update.
```

Errors must wrap the offending field name (`fmt.Errorf("%w: %q", ErrFieldNotFilterable,
name)`) so consumers can produce a useful 400 message.

---

## 4. Tag grammar (extend `internal/tag`)

Extend `r3tag.ParseColumnTag` (`internal/tag/tag.go`) with additive capability flags. All
existing tags keep working. New `ColumnTag` fields + flags:

| Tag flag        | Effect                                            |
|-----------------|---------------------------------------------------|
| `no-filter`     | clear `Filterable`                                |
| `no-sort`       | clear `Sortable`                                  |
| `no-output`     | clear `Queryable` (hidden from SELECT & output)   |
| `readonly`      | clear `Creatable` **and** `Mutable`               |
| `immutable`     | clear `Mutable` only (set-once)                   |
| `enum:a|b|c`    | `Type=TypeEnum`, `Enum=[a,b,c]`                    |

Examples:

```go
ID        int64     `r3:"id,pk"`                       // → readonly, queryable, sortable, filterable
CreatedAt time.Time `r3:"created_at"`                  // → readonly by default-rule (timestamp)
Title     string    `r3:"title"`                       // → all caps
Status    string    `r3:"status,enum:draft|planned|completed|published"`
Secret    string    `r3:"secret_token,no-filter,no-sort,no-output"`
Slug      string    `r3:"slug,immutable"`              // creatable, not mutable
Population int      `r3:"population,readonly"`          // feed-synced; users can't write; worker can via bypass
```

`isKnownKeyword`/`applyFlag` in `internal/tag/tag.go` must learn the new flags. Keep the
"first part may be a column name or a keyword" logic intact.

---

## 5. Implementation phases

Land in this order. Each phase is independently reviewable and (A, B) ships behavior that
is strictly safer than today.

### Phase A — Core types + derivation (no behavior change)

**Goal:** the `r3.Schema`/`Attribute` model exists and can be derived from any struct,
with correct default capabilities. Nothing consumes it yet.

Files:
- `r3/schema.go`, `r3/schema_caps.go` — types, `Capability`, `DataType`, `Has`, lookups.
- `r3/schema_derive.go` — `SchemaOf[T]`, default-capability policy (§2.7), per-type cache.
- `r3/schema_validate.go` — `ValidateQuery`, `Filterable/Sortable/Queryable/Writable`.
- `internal/tag/tag.go` — new capability flags (§4).
- `r3_errors.go` — new typed errors (§3.1).
- Default operator sets per `DataType` (string → eq/ne/in/nin/like/ilike/exists;
  numeric/time → + gt/gte/lt/lte/between*; bool → eq/ne; enum → eq/ne/in/nin).

Tests:
- `schema_derive_test.go`: a model with every tag combination → asserted caps, type
  inference (Go kind → DataType), PK/timestamp/soft-delete default exceptions, relations.
- `schema_validate_test.go`: ValidateQuery accepts allowed fields; rejects
  non-filterable/non-sortable/non-queryable/unknown with the right typed error wrapping the
  field name.
- Round-trip: `SchemaOf` is deterministic and order-stable.

Acceptance: `go test ./...` green; **zero** changes to existing engine/driver behavior.

### Phase B — Engine enforcement (reads + writes) + escape hatch

**Goal:** turn unknown-field SQL 500s into typed errors, and make writes honor
`Creatable`/`Mutable`. This is where the value lands.

#### B.1 Read validation
- In `engine/sql.PrepareMergedListQuery` (`engine/sql/list_query.go`), before converting
  filters/sorts/fields to SQL, validate them against the entity schema and return the typed
  error. The schema must be reachable here — pass it in (preferred) or look it up via the
  driver. **Preferred wiring:** the driver builds the schema once (see B.3) and calls a new
  `PrepareMergedListQueryWithSchema(q, schema)` (keep the old signature as a thin wrapper
  with a nil schema = no validation, for back-compat / non-SQL callers).
- Relation-filter fields (the `has`/relationship path in `drivers/gorm/relfilter.go`) must
  be validated against the *target* entity's schema, not the root — keep that in mind, but
  if it's awkward in this phase, validate only root-level fields and `// TODO` the relation
  case rather than reject valid relation filters.

#### B.2 Write enforcement (engine)
- Generalize `StructMeta.ValidatePatchColumns` → schema-aware: a Patch field must be
  `Mutable`; a non-`Mutable` field named in a Patch is `ErrInvalidPatchField`. (Subsumes
  today's PK + soft-delete rejection.)
- **Update SET-shaping (the bug fix):** Update must write only `Mutable` columns. Implement
  by filtering the column set used by the SQL Update (`engine/sql` Update path and each
  driver's Update). For GORM specifically, prefer an explicit `Select(mutableCols)` so
  protected columns can't be written even if non-zero. Verify `created_at` is never
  clobbered and a soft-deleted row isn't resurrected by a full Update.
- Create: exclude non-`Creatable` columns from INSERT (server fills PK/timestamps).

#### B.3 Schema on the repo + caching
- Each SQL driver builds its schema **once** at construction (today `GetStructMeta[T]()` is
  re-reflected on every op — fix that too by caching `StructMeta` per type). Add a
  package-level `sync.Map` cache keyed by `reflect.Type` for both `StructMeta` and the
  derived `r3.Schema`, or store the schema on the CRUD struct in `NewGormCRUD` (and bun/
  gopg/sqlite3/pq/mongo equivalents).
- The engine binding links logical attribute name → column via `StructMeta.Columns`. Add a
  startup-time consistency check (every logical attr resolves to a real column/relation);
  fail fast on mismatch.

#### B.4 Escape hatch (system/worker writes) — keep audit
The existing `GormCRUD.Raw() *GormRaw` is the **bare-SQL** path (no engine features, no
audit). It stays as-is and is documented as the blunt knife (`// bypasses ALL features;
migrations/repair only`). It is **not** the everyday worker door, because it loses history.

For the audited worker bypass, enforcement (B.2) checks an **explicit per-call context
marker** at the engine seam:

```go
// r3_schema_bypass.go (core)
func WithoutWriteGuard(ctx context.Context) context.Context  // explicit, per-call
func writeGuardBypassed(ctx context.Context) bool
```

The engine's write-cap enforcement is skipped when `writeGuardBypassed(ctx)` is true; the
**structural floor (§2.4) still applies** (computed/PK never writable). Because the marker
rides on `ctx`, it flows *down* through the `history`/`metrics` decorators first, so worker
writes are still audited — then the engine skips the capability check. This is the
"guard is always there; opt out explicitly" model.

Ergonomic wrapper (recommended, reads like the chained form the team liked without
colliding with the bare `Raw()`):

```go
// SystemWriter wraps the TOP of a decorator chain; its write methods inject
// WithoutWriteGuard(ctx) before delegating, so the full (audited) chain runs but
// the engine skips capability checks. Reads/structural floor unaffected.
func SystemWriter[T any, ID comparable](repo CRUD[T, ID]) CRUD[T, ID]
// usage: r3.SystemWriter(repo).Update(ctx, feedRow)
```

> Open decision for the implementer (flag in the PR, don't block on it): whether to also
> offer a chained `repo.Unguarded()` handle. The context-marker + `SystemWriter` approach is
> preferred because enforcement lives innermost (engine), where a rewrapped "chain minus a
> layer" handle is awkward, and because it provably preserves audit. Do **not** reuse the
> name `Raw()` for the audited bypass — it's taken by the bare path.

Tests (Phase B):
- List/Count with a non-filterable field → typed error, **no SQL emitted** (assert via a
  failing/fake driver or by checking the error type before DB).
- Sort on a non-sortable field, SELECT of a non-queryable field → typed errors.
- Update does not clobber `created_at`; Update on a soft-deleted row doesn't resurrect it.
- Patch of a `Mutable=false` field → `ErrInvalidPatchField`.
- `SystemWriter(repo).Update(...)` writes a `readonly` (feed) column **and** records a
  history entry (wire the `history` decorator in the test to assert the audit row).
- `SystemWriter(repo)` still cannot write a computed/PK (structural floor holds).
- Cross-driver: replicate the read-validation + write-shaping tests for at least gorm +
  one raw SQL driver (sqlite3) to prove the engine seam (not the driver) carries it.

Acceptance: existing suites green; new behavior covered; the only *intentional* behavior
change is "unknown field → typed error instead of SQL error" and "Update no longer writes
protected columns" — both call-outs in the changelog.

### Phase C — Introspection serialization (still R3-only)

**Goal:** R3 can serialize a `Schema` to a stable JSON shape, so a consumer can later build
a `/-/schema` endpoint and dynamic UI. No HTTP here — just the dialect.

- Add `dialects/json` (or a new `dialects/schema`) functions:
  `MarshalSchema(s r3.Schema) ([]byte, error)` producing a documented, versioned JSON shape
  describing each public attribute: `name`, `type`, `caps` (as booleans
  `filterable/sortable/queryable/creatable/mutable`), `ops` (operator names), `enum`,
  `relation` (target + label), `computed`.
- **Serialize only the public projection**: attributes without `Queryable` are omitted; the
  bypass/system door is never advertised.
- Include the reserved `computed` flag from day one so the JSON contract is
  forward-compatible when §8 lands.

Tests: golden-file JSON for a representative model; assert non-queryable attrs are absent;
assert operator lists match the type defaults.

Acceptance: stable, documented JSON; `readme.md`/`doc.go` updated.

---

## 6. Escape-hatch summary (the two knives)

| Call                                   | Strips                              | Keeps                       | Use |
|----------------------------------------|-------------------------------------|-----------------------------|-----|
| `r3.SystemWriter(repo)` / `WithoutWriteGuard(ctx)` | write-capability checks (`Creatable`/`Mutable`) | structural floor, history, metrics, soft-delete | everyday worker/feed-sync write of a user-immutable field, **audited** |
| `repo.Raw()` (bare `GormRaw`)          | **everything** — straight to SQL    | nothing                     | migrations, data repair, "I know exactly what I'm doing" |

The **structural floor is un-bypassable by both** — a computed/PK has no writable column.

---

## 7. Back-compat & migration notes

- `StructMeta` stays. `r3.Schema` is **derived from the same tags** and is the new logical
  source of truth; the engine keeps using `StructMeta` for the physical column binding.
  (A later cleanup may make core `Schema` the single source that engines build `StructMeta`
  *from*; do **not** attempt that big-bang refactor in these phases.)
- `ValidatePatchColumns` keeps its signature; reimplement its body in terms of the schema
  (or keep it and add the new `Mutable` check) so existing callers are unaffected.
- `PrepareMergedListQuery` keeps its current signature (nil-schema = no validation); add the
  schema-aware variant alongside. No driver outside R3 should break.
- The two intentional behavior changes (unknown-field → typed error; Update no longer writes
  non-`Mutable` columns) are **pre-1.0, documented** breaks. Note them in `tasks.md` /
  changelog with before/after.
- Keep depguard/`.golangci.yml` layering intact: core `r3` package must stay
  **dependency-free** (no imports of `engine`/`dialects`/`drivers`). `schema*.go` files use
  only stdlib + `internal/utils`, `internal/tag`.

---

## 8. Out of scope (future phases — design-reserve only)

- **Computed / aggregation attributes (`Computed`, `Compute`).** Reserve the `Attribute`
  field and the JSON flag now (so the contract is stable), but do **not** implement
  execution. Future work: portable primitives (`Count(relation)`, `Sum`, `Avg`) compiled
  per engine + a SQL-only raw-expr escape; sorting/filtering on computed attrs.
  Note: *query-time* aggregation (GROUP BY/HAVING over a repo's own rows) has since
  shipped as the opt-in `r3.Aggregator` capability (`Query.GroupBy`/`Aggregates`/
  `Having`, validated by `Schema.ValidateAggregateQuery`); what remains reserved here
  is aggregation as a schema *attribute* of the entity itself.
- **`DefaultsStrict()` policy** and **per-operator validation** (reject `gt` on a bool):
  the operator metadata (`Ops`, per-type defaults) is built in Phase A, but enforcing it in
  the validator can be a fast-follow if it risks rejecting currently-working queries — gate
  it behind a config flag first.
- **Mongo/other-engine bindings.** The core schema is engine-neutral; only `engine/sql` is
  wired here. A Mongo binding is a separate effort.
- **The p44 side entirely**: `/-/schema` HTTP endpoint, schema-aware `ParseListQuery` → 400,
  URL↔filter sync, dynamic admin tables.

---

## 9. Definition of done (this plan, R3-only)

1. `r3.Schema`/`Attribute` exist in the core package, dependency-free, with cached
   tag-derivation and documented default capabilities.
2. A bad filter/sort/select field produces a **typed error before SQL**, across the SQL
   engine (proven on gorm + sqlite3).
3. `Creatable`/`Mutable` are enforced on Create/Update/Patch; `created_at` can no longer be
   clobbered by Update.
4. An explicit, **audited** per-call bypass (`SystemWriter`/`WithoutWriteGuard`) lets worker
   code write user-immutable columns; the structural floor still blocks computed/PK.
5. A `Schema` serializes to a stable, public-only JSON shape (computed flag reserved).
6. `go test ./...` green; new tests cover the matrix in §5; `readme.md`/`doc.go`/`tasks.md`
   updated; lint/depguard clean.
</content>
</invoke>
