# Plan: Upsert + Bulk conditional update (Tier-2 commands)

Status: **MVP implemented** (both capabilities). Landed: core interfaces +
helpers + sentinels (`r3_upsert.go`, `r3_bulkpatch.go`, `r3_errors.go`); engine
impls for `drivers/gorm` and `engine/sql` (inherited by pq/pgx/mysql/sqlite3);
forwarding through all seven decorators; and tests (core helpers, gorm + raw
sqlite3 backends, permissions scope/dual-check, history create-vs-update &
per-row audit). **Not yet done:** the `mongo`/`file`/`bun`/`gopg` backends (each
absent impl correctly degrades to the `…NotSupported` sentinel), the p44
adoption sites (§A.6/§B.4), and canonical/serialization wiring. Authored from the
p44 side (the first real R3 consumer). Two new *write* capabilities that p44
currently works around with raw GORM.

Both follow one non-negotiable rule (see "Conventions" below): they are **opt-in
capability interfaces + package-level helpers**, exactly like `Aggregator` /
`AggregateOf`. They are **NOT** added to the core `Commander` interface - doing
so would break every engine, driver, decorator, and third-party backend at once.

---

## Why (consumer motivation)

- **Upsert** - p44's `svcsettings.Set` is a key-value write keyed by a string PK.
  There is no `Upsert`, so it stays on raw `gorm clause.OnConflict`; the r3
  alternative (Get-then-Create/Update) races. Any future KV/settings/config store
  hits the same wall.
- **Bulk conditional update** - p44's boot-recovery sweeps ("mark every run still
  `running` as `interrupted`") and `resetStuckProcessing` ("photos WHERE
  status=processing → pending") are `UPDATE ... WHERE <filter>`. `Patch` only
  updates one row by PK, so these are either raw `Updates` or a List-then-Patch
  loop (fine at boot over a handful of rows, wrong at scale).

Priority: **do Upsert first** (smaller, unblocks a clean p44 site); bulk update
second.

---

## Conventions to follow (from the existing `Aggregator` capability)

Mirror this pattern precisely - it is the established R3 way to add an optional
capability without breaking anyone:

1. **Optional interface** in the core package (see `r3_aggregate.go` `Aggregator`).
2. **Package helper** that type-asserts and returns a `…NotSupported` sentinel
   otherwise (see `AggregateOf` + `ErrAggregateNotSupported` in `r3_errors.go`).
3. **Capability detection through decorators** via `Unwrapper` (`r3_unwrap.go`):
   each decorator implements `Unwrap()`, and forwards the capability by
   asserting on its `inner` (see `features/history/decorator.go:153`,
   `features/permissions/decorator.go:252`, `features/i18n/decorator.go:115`).
4. Each decorator carries a `var _ r3.Aggregator = &CRUD[...]{}` compile-time
   assertion (e.g. `features/history/decorator.go:43`). Add the equivalent for
   the new interfaces.
5. Enforce schema capabilities (`schema_caps.go`: `Creatable` gates inserts,
   `Mutable` gates updates) and managed timestamps (created_at on insert,
   updated_at on any write), honoring the `WriteGuard`/`SystemWriter` bypass door
   the same way `Create`/`Patch` do (see `drivers/gorm/gormcrud.go` `Patch`,
   `engine/sql/crud.go` `stampManagedTimestamps`).

---

## Part A - Upsert (do first)

### A.1 Core interface + helper - new file `r3_upsert.go`

```go
// Upserter is an optional capability: insert the entity, or update it in place
// when it collides on the conflict target. Backends opt in; use UpsertOf.
type Upserter[T any, ID comparable] interface {
    Upsert(ctx context.Context, entity T, opts ...UpsertOption) (T, error)
}

type UpsertSpec struct {
    // ConflictColumns is the conflict target. Empty = the primary key.
    ConflictColumns []string
    // UpdateFields are the columns overwritten on conflict. Empty = all mutable
    // columns of the entity (i.e. "replace").
    UpdateFields Fields
}

type UpsertOption func(*UpsertSpec)

// OnConflict sets the conflict target columns (default: PK).
func OnConflict(cols ...string) UpsertOption { /* … */ }
// UpdateOnConflict restricts which columns are written on conflict.
func UpdateOnConflict(fields ...*FieldSpec) UpsertOption { /* … */ }

// UpsertOf runs an upsert if repo (or anything it unwraps to) supports it.
func UpsertOf[T any, ID comparable](
    ctx context.Context, repo Commander[T, ID], entity T, opts ...UpsertOption,
) (T, error) {
    up, ok := repo.(Upserter[T, ID])
    if !ok {
        var zero T
        return zero, ErrUpsertNotSupported
    }
    return up.Upsert(ctx, entity, opts...)
}
```

Add `var ErrUpsertNotSupported = errors.New("r3: upsert not supported")` to
`r3_errors.go` (next to `ErrAggregateNotSupported`).

Note: unlike `Aggregator` (non-generic, returns `[]AggregateRow`), `Upserter`
returns `T`, so the interface is generic `[T, ID]` and `UpsertOf` asserts
`repo.(Upserter[T, ID])`. That's fine - the concrete type is known at the call
site.

### A.2 Semantics

- Insert path obeys `Creatable`; on-conflict update path obeys `Mutable`
  (reuse `RequireMutableColumns`). Reject an `UpdateOnConflict` that names a
  non-mutable/read-only column, same error as `Patch`.
- Stamp managed timestamps: created_at on the insert branch, updated_at always.
- Return the full row after the write (re-fetch like `Patch` does, or use
  RETURNING where the backend supports it).

### A.3 Per-backend implementations (implement in this order)

1. **`drivers/gorm`** (`GormCRUD.Upsert`) - the p44-critical one. Straight
   `clause.OnConflict{Columns: conflict, DoUpdates: clause.AssignmentColumns(update)}`
   then `Create(&entity)`. This is literally the code p44 hand-writes today.
2. **`engine/sql` (`BaseCRUD.Upsert`)** - the reference impl for the raw SQL
   drivers. Emit dialect-appropriate SQL via the flavor layer:
   - Postgres / SQLite: `INSERT … ON CONFLICT (cols) DO UPDATE SET …`
   - MySQL: `INSERT … ON DUPLICATE KEY UPDATE …`
   Reuse `createColumns` / `updateColumns` / `stampManagedTimestamps`.
3. **`engine/mongo`** - `ReplaceOne(filter{conflict cols → entity values}, entity,
   options.Replace().SetUpsert(true))`, or `UpdateOne` with `$set` when
   `UpdateFields` is a subset.
4. **`engine/file`** - keyed put: if a record matching the conflict target exists,
   replace (or merge `UpdateFields`); else create (via the `IDGenerator`).
5. **`drivers/bun`, `gopg`, `pq`, `pgx`, `mysql`, `sqlite3`** - map to each lib's
   upsert; the raw drivers can share `engine/sql`'s SQL builder.

MVP for p44 = interface + helper + **gorm + engine/sql + decorators + tests**.
The other backends can follow incrementally (p44 only uses the gorm driver), but
each unimplemented backend must simply *not* satisfy `Upserter` so `UpsertOf`
returns `ErrUpsertNotSupported` cleanly - never a panic.

### A.4 Decorator forwarding (all of `features/`)

For each decorator (`history`, `permissions`, `i18n`, `metrics`, `softdelete`,
`validation`, `transactor`): add `var _ r3.Upserter[…] = …`, and forward:

```go
func (d *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
    up, ok := d.inner.(r3.Upserter[T, ID])
    if !ok { var zero T; return zero, r3.ErrUpsertNotSupported }
    // … decorator-specific pre/post work …
    return up.Upsert(ctx, entity, opts...)
}
```

Decorator-specific behavior (call these out in review):
- **permissions** - an upsert is create-*or*-update, so it must pass **both** the
  create and the update capability check (gate on the stricter). A `Scoper`, if
  present, must not let the write target rows outside scope.
- **history** - needs a pre-state to diff. Simplest correct behavior: fetch the
  row by conflict target first; if present record an update-diff, else record a
  create. Document the chosen behavior; a "record after the fact without diff"
  fallback is acceptable if fetch-before is too costly, but say so.
- **i18n** - mark translations of changed source fields stale, same as
  Create/Update.
- **validation** - run the validator on the entity, same as Create/Update.

### A.5 Tests

- Per-backend: insert-when-absent; update-when-present; custom `OnConflict`
  columns; `UpdateOnConflict` subset; capability rejection (non-mutable update
  column); `ErrUpsertNotSupported` from a backend that doesn't implement it.
- Decorators: permissions denies without create+update cap and respects Scoper;
  history records the right create-vs-update entry; i18n stale-marks.

### A.6 p44 adoption (after an R3 tag + pin bump - not part of this task)

`svcsettings.Set` →
`r3.UpsertOf(ctx, s.repo, setting, r3.OnConflict("key"))`, dropping the raw
`clause.OnConflict` and the `db` field. Remove the "commands: upsert" item from
p44's `R3_TODO.md`.

---

## Part B - Bulk conditional update / "update-where" (do second)

### B.1 Interface + helper - `r3_bulkpatch.go`

```go
// BulkPatcher is an optional capability: set the named fields (to the entity's
// values) on every row matching filters. Returns the affected-row count.
type BulkPatcher[T any, ID comparable] interface {
    PatchWhere(ctx context.Context, filters Filters, entity T, fields Fields) (int64, error)
}

func PatchWhereOf[T any, ID comparable](
    ctx context.Context, repo Commander[T, ID], filters Filters, entity T, fields Fields,
) (int64, error) {
    bp, ok := repo.(BulkPatcher[T, ID])
    if !ok { return 0, ErrBulkPatchNotSupported }
    return bp.PatchWhere(ctx, filters, entity, fields)
}
```

Add `ErrBulkPatchNotSupported` to `r3_errors.go`. `PatchWhere` mirrors `Patch`
semantics (partial, by `Fields`) but selects rows by `Filters` instead of PK.

### B.2 Per-backend

- **gorm / engine/sql**: `UPDATE <table> SET <fields> WHERE <filters>` - gorm:
  `Model(&T).Where(<translated filters>).Select(cols).Updates(entity)`. Reuse the
  existing filter→SQL translation (`PreparedListQuery` / `buildFilterSQL`).
- **mongo**: `UpdateMany(filter, {$set: …})`.
- **file**: load collection, filter in memory, patch each match, rewrite.
- Enforce `Mutable` caps + managed updated_at, as `Patch` does.

### B.3 Decorators - the thorny parts (flag prominently)

- **permissions** - the bulk update MUST AND-in the `Scoper` filters, or a scoped
  actor could mutate rows outside its scope. This is a security-critical detail:
  the effective filter is `caller filters AND scoper filters`. Also gate on the
  update capability.
- **history** - a bulk update is N row changes with no cheap per-row pre-state.
  Recommended: for an audited repo, pre-`List` the matching ids and record a
  change per row (bounded - callers use this for small sweeps); OR explicitly
  document that `PatchWhere` bypasses history and `log`/return a signal so a
  caller can't silently lose an audit trail. Pick one and make it loud.

### B.4 Tests + p44 adoption

- Tests: matches update / non-matches untouched / affected count / Scoper
  confinement / Mutable rejection / `ErrBulkPatchNotSupported`.
- p44 (post-release): `svcstylize.RecoverInterruptedRuns`,
  `svcimport.RecoverInterruptedRuns`, and `svcstylize.resetStuckProcessing`
  collapse from List-then-loop / raw `Updates` to a single `PatchWhere`. Remove
  the "commands: bulk conditional update" item from p44's `R3_TODO.md`.

---

## Sequencing summary

1. **Upsert MVP**: `Upserter` + `UpsertOf` + `ErrUpsertNotSupported`; gorm +
   engine/sql impls; all decorators forward; tests. → tag a minor release.
2. **Bulk update**: `BulkPatcher` + `PatchWhereOf` + error; gorm + engine/sql;
   decorators (with the permissions/history nuances resolved); tests. → tag.
3. Remaining backends (mongo/file/bun/gopg/pq/pgx/mysql/sqlite3) incrementally;
   each absent impl must degrade to the `…NotSupported` sentinel, never panic.

## Key files to touch

- New: `r3_upsert.go`, `r3_bulkpatch.go`; additions to `r3_errors.go`.
- Impls: `drivers/gorm/gormcrud.go`, `engine/sql/crud.go` (+ other drivers/engines).
- Decorators: `features/{history,permissions,i18n,metrics,softdelete,validation,transactor}/decorator.go`.
- Templates to copy from: `r3_aggregate.go` (`Aggregator` + `AggregateOf`),
  `r3_unwrap.go`, and each decorator's existing `Aggregator` forwarding.
