# Plan: Permissions advertisement (capability probe + projection)

Status: **Implemented** (shipped as one step; see "Review adjustments" below).
Authored from the p44 side (the first real R3 consumer). Purely additive inside
`features/permissions`: no change to `crud.go`, no change to the decorator's
enforcement behaviour, zero breaking surface. It promotes the permissions
feature from *enforce-only* to *enforce + advertise*, so a caller can ask
"would this operation be allowed?" without performing it - and get an answer
computed by the **same** `Checker`/`Scoper` that enforces, so the two can never
drift.

## Review adjustments (2026-07-15, adopted before implementation)

1. **Parts A, B, C ship together as one step.** The plan itself calls a
   Check-only read probe wrong; shipping it even for one tag is a trap, and
   Part B is ~10 lines reusing `match.go`. There is no phased rollout.
2. **`AllowResource` added to the API.** Without a row-less probe, consumers
   would pass a zero-value `T` for questions like "show the New button?" and an
   entity-reading policy would return a confidently wrong verdict. It builds
   the decorator's canonical resource-level request (`Entity` and `EntityID`
   both nil - the same shape `List`/`Count` issue).
3. **The EntityID divergence is documented and tested, not hidden.** The
   decorator's `Delete` always sets `EntityID` (even without `WithIDFunc`);
   package-level `Allow` never does. A Checker that keys decisions on
   `req.EntityID` must use the decorator methods (`(*CRUD).Allow`), which
   populate it via `IDFunc`. The parity test encodes this boundary.
4. **Factual fix:** "writes are never authorised by the read Scoper" was
   overstated - `Upsert` and `PatchWhere` (writeext.go) do consult the Scoper.
   It is true only for the plain `Update`/`Patch`/`Delete` paths that `Allow`
   mirrors.
5. **Naming stays `Allow` / `AllowedOps`** (house `Allow*` prefix), joined by
   `AllowResource`.

---

## Why (consumer motivation)

p44 renders an admin UI whose controls must match what the backend will permit.
Today the backend enforces correctly (the decorator denies), but the frontend
has **no way to know the verdict in advance**, so it either:

1. **re-implements the policy in TypeScript** (`useAuth.ts` mirrors `CapFor`,
   already documented there as "slightly permissive" - i.e. drifting), or
2. **gates on the wrong, coarsest flag.** The concrete bug that triggered this
   plan: the "Location Lab" (edit) button is shown on `v-if="isAdmin"` where
   `isAdmin` means merely "can enter the admin panel". A squad-scoped editor,
   or an activist who owns a different row, sees an edit control the backend
   then rejects. The click fails closed - correct, but a bad UX and a sign the
   client is guessing.

The root cause is architectural: **the authority decision lives server-side, but
there is no server-side way to publish it per row.** Row-level verdicts
(ownership: "this raid is mine because I lead it"; squad scope) *cannot* be
computed on the client without shipping the whole policy plus every owner id -
which is exactly the drift we want to kill.

What p44 needs from R3 is small: given a `Checker` (the policy it already
registers) and a row it already holds, tell it which operations the current
actor may perform on that row. p44 then projects that into a compact per-row
`can` block on its DTOs (`{"update": true, "delete": false}`), and the frontend
becomes a flag-renderer instead of a policy re-implementation.

R3's job stays generic and CRUD-only. "Lab" is a p44 affordance; R3 answers
`update: allowed` and p44 maps "Lab shows when update is allowed". The framework
must not learn app vocabulary.

---

## The core idea

The `Checker` is **already a pure, side-effect-free probe**:

```go
err := checker.Check(ctx, AccessRequest[T, ID]{Operation: OpUpdate, Actor: a, Entity: &row})
// nil => would be allowed; *AccessDeniedError => would be denied
```

So the primitive exists at the interface level. Two things are missing, and both
are ergonomics/correctness rather than new mechanism:

1. **A tidy affirmative API** so callers don't hand-assemble an `AccessRequest`
   per op (and don't have to remember the Entity/EntityID nil-conventions the
   decorator uses). Advertisement wants `bool`, not `error`.

2. **Faithfulness to the decorator's *real* decision, including the Scoper.**
   This is the subtle part. The decorator's read path is **not** just
   `Check(OpRead)` - `Get` also runs the `Scoper` and masks an out-of-scope row
   as `ErrNotFound`. A naive "just call Check" read-probe would therefore be
   *wrong* (it would claim readable a row the decorator hides). The advertise API
   must mirror the same composition: `Check` for writes, `Check` **and** scope
   for reads. R3 already has the in-memory matcher for this (`match.go`), so we
   reuse it rather than re-derive it.

The design principle: **one decision function, two entry points.** Enforcement
(the decorator) and advertisement (these helpers) must route through identical
logic. The plan encodes that as a parity test (§Tests), not just a convention.

---

## Conventions to follow

Mirror the established R3 patterns (same discipline as `plan-upsert-and-bulk-update.md`):

1. **Package-level helpers over a `Checker`**, in the `permissions` package,
   styled like `AllowAll` / `Compose` / `OperationCheckers` in `helpers.go`.
   Package-level (not only decorator methods) is a hard requirement, see below.
2. **No new core interface, no `Commander`/`Querier` change.** This lives
   entirely in `features/permissions`.
3. **Actor comes from context** via `r3.GetActor(ctx)`, exactly as the decorator
   reads it - so a helper and the decorator always see the same actor.
4. **Reuse, don't re-derive.** Read-scope evaluation reuses `entityMatchesFilters`
   and `containsRelationFilter` from `match.go`, with the same fail-closed rule
   the decorator's `Get` uses for relation ("has") filters.
5. **Compile-time assertions + table tests** in the feature's existing test
   style (`decorator_test.go`, `scope_pointer_test.go`).

### Why package-level (not just a decorator method) is mandatory

p44's projector runs in the **DTO/handler layer**, holding the policy object
(`permissions.Checker`), and it must run even on the **public map endpoint**,
which does **not** go through the permissions decorator at all (that repo is
unwrapped). So the probe cannot require a `*permissions.CRUD` instance. It must
operate on a bare `Checker` (+ optional `Scoper`), decoupled from the repo
wrapper. A decorator convenience method (§Part C) is a thin sugar on top, for
callers who *do* hold the decorator.

---

## Part A - Op advertisement (shipped with B and C)

The minimum that fixes the consumer bug. Pure, in-memory, no I/O.

New file `features/permissions/advertise.go`:

```go
// Allow reports whether op would be permitted for the context actor on the
// given (already-loaded) entity, WITHOUT performing it. It asks the same
// Checker the decorator asks, building the same AccessRequest the decorator's
// corresponding method builds - so an advertised verdict matches enforcement.
//
// It is a projection primitive: use it to publish per-row capabilities (e.g. a
// DTO "can" block a frontend reads to show/hide controls). The backend still
// enforces on every real operation; this only advertises the verdict.
//
// Scope: for OpRead, Allow additionally consults the Scoper (if the checker is
// one), matching the decorator's Get - an out-of-scope row is not readable. See
// AllowRead notes on relation ("has") filters. For write ops, only Check runs:
// the plain Update/Patch/Delete paths Allow mirrors never consult the read
// Scoper. (The write extensions Upsert/PatchWhere DO consult it, but they are
// compositions - probe them as OpCreate+OpUpdate / OpUpdate respectively.)
func Allow[T any, ID comparable](
    ctx context.Context, checker Checker[T, ID], op Operation, entity T,
) bool

// AllowedOps returns the subset of ops permitted for the context actor on
// entity, preserving the given order. Convenience over Allow for building a
// capability set to serialise. With no ops given, it probes the full CRUD set
// (OpCreate, OpRead, OpUpdate, OpDelete).
func AllowedOps[T any, ID comparable](
    ctx context.Context, checker Checker[T, ID], entity T, ops ...Operation,
) []Operation

// AllowResource reports whether the actor passes the resource-level check for
// op: Entity and EntityID nil, the exact request List/Count issue for OpRead.
// Use it for row-less questions ("show the New button?", nav gating) instead of
// probing Allow with a zero-value entity, which an entity-reading policy would
// judge on garbage field values.
func AllowResource[T any, ID comparable](
    ctx context.Context, checker Checker[T, ID], op Operation,
) bool
```

`Allow` builds the `AccessRequest` per op with the **same field conventions the
decorator uses** (this is the anti-drift contract):

| op         | Entity        | EntityID | extra                          |
|------------|---------------|----------|--------------------------------|
| OpCreate   | `&entity`     | nil      | Check only                     |
| OpRead     | `&entity`     | nil*     | Check **and** Scoper (Part B)  |
| OpUpdate   | `&entity`     | nil*     | Check only                     |
| OpDelete   | `&entity`     | nil*     | Check only                     |

\* EntityID is left nil in the MVP. p44's policies decide writes from
`req.Entity` (squad/owner fields), never from `req.EntityID`, so Entity-only is
faithful for the consumer. Populating EntityID faithfully requires an id
extractor; offered as an opt-in in Part C for Checkers that read EntityID.

Note the decorator's `Update`/`Delete`, when `WithIDFunc` is set, check against
the **fetched existing** row, not the incoming one. In advertisement the caller
passes the row they already hold - which *is* that existing row (it came from a
prior scoped List/Get). So passing the loaded entity is consistent by
construction; `Allow` performs no fetch.

Return-shape rationale: `Allow` returns `bool` (advertisement discards the deny
*reason*; `Check` remains available when a reason is wanted). `AllowedOps`
returns `[]Operation` (ordered, allocation-light, trivial to test and to map to
a JSON object on the consumer side).

Naming: `Allow` / `AllowedOps` matches the `AllowAll` family. Alternatives
considered: `Can`/`Permits` (rejected: `Allow*` is the house prefix). Final call
left to the maintainer at implementation.

---

## Part B - Scope-aware read advertisement (shipped with A)

`Allow(OpRead, entity)` must equal the decorator's `Get` verdict, which is
`Check(OpRead)` **AND** in-scope. Implement by folding the Scoper in, reusing
the existing matcher:

```go
// inside Allow, for op == OpRead, after a passing Check:
if scoper, ok := checker.(Scoper[T, ID]); ok {
    filters, err := scoper.Scope(ctx, r3.GetActor(ctx))
    if err != nil { return false }          // fail closed
    if len(filters) > 0 {
        if containsRelationFilter(filters) {
            return false                     // can't match "has" in memory; fail
        }                                    //   closed, exactly like Get w/o IDFunc
        inScope, matchErr := entityMatchesFilters(&entity, filters)
        if matchErr != nil || !inScope { return false }
    }
}
```

Consistency note: this reproduces `decorator.go` `Get`'s behaviour precisely -
plain column filters matched in memory, relation filters fail closed. The one
divergence: the decorator can *recover* a relation-filtered `Get` via a backend
query when `WithIDFunc` is set; the pure advertise path deliberately does **no**
I/O and stays fail-closed. That is the correct trade for a projection primitive
(advertising must be cheap and side-effect-free); a caller that needs relation
scope in the read verdict should scope the *List* that produced the rows (so
every returned row is in-scope by construction, making read-advertise trivially
true). Document this on `Allow`.

(Originally phased as a follow-up; folded into the single shipment per review
adjustment 1 - a Check-only read verdict is knowingly unfaithful and the
implementation is trivial given `match.go`.)

---

## Part C - Decorator convenience + optional EntityID (shipped with A)

For callers who hold a `*permissions.CRUD` (not the bare Checker), add thin
sugar that also has access to `opts.IDFunc`:

```go
// Allow reports whether op would be permitted on entity, using this decorator's
// checker (and IDFunc, if set, to populate AccessRequest.EntityID faithfully).
func (p *CRUD[T, ID]) Allow(ctx context.Context, op Operation, entity T) bool

func (p *CRUD[T, ID]) AllowedOps(ctx context.Context, entity T, ops ...Operation) []Operation
```

These delegate to the package helpers, adding `EntityID` from `p.opts.IDFunc`
when present - so a Checker that *does* read `req.EntityID` gets it. Keep the
package-level helpers as the primitive; these are ergonomic wrappers.

---

## Tests

Mirror `decorator_test.go` / `scope_pointer_test.go` style. Three groups:

1. **Unit** - a table-driven policy (a `Checker` with a couple of actor types)
   asserting `Allow`/`AllowedOps` per (actor, op, entity), including the
   ownership-style row-level case (allow update on an owned row, deny on a
   foreign one) that motivated the plan.

2. **Scope** (Part B) - a `Scoper` policy: `Allow(OpRead, e)` true for an
   in-scope row, false for out-of-scope; a relation-filter Scoper fails closed.
   Reuse the fixtures from `scope_pointer_test.go`.

3. **Parity (anti-drift) - the important one.** For a matrix of (actor, op,
   entity), assert that `Allow(ctx, checker, op, e)` agrees with actually
   driving the decorator: `Allow == true` iff the decorator's real
   `Create/Get/Update/Delete` on `e` does **not** return an access error (using
   an `AllowAll` inner backend so the only possible failure is the permission
   layer). This encodes "advertisement can't drift from enforcement" as a test,
   so a future change to one path that forgets the other fails CI.

`just lint` clean; `go test -short ./...` green (all in-memory, no Docker).

---

## Non-goals

- **No app affordances in R3.** R3 speaks `OpCreate/Read/Update/Delete`. "Lab",
  "publish", "moderate" are consumer mappings over CRUD verdicts. If a real
  affordance ever stops mapping 1:1 to a CRUD op, `Operation` is already a
  `string` type and a project can define custom ops - but that is out of scope
  here and should not be added speculatively.
- **No HTTP / DTO / serialization layer.** R3 returns `[]Operation`; the
  consumer shapes the wire (`can` block, field names, `omitempty`).
- **No batch/list helper in R3.** Annotating a slice is a trivial
  `for`-loop over `AllowedOps` on the consumer side; R3 need not own it.
- **No caching / memoization.** Probes are cheap (in-memory Check + optional
  in-memory filter match). Callers that fan out over large lists can memoize by
  actor if ever needed.
- **No change to enforcement semantics.** The decorator behaves exactly as
  today; this only adds read-only query helpers beside it.

---

## Consumer (p44) consumption sketch - for context only, not part of this repo

```go
// p44, DTO/handler layer, holding the same policy object it registers in app.go:
ops := permissions.AllowedOps[Location, int64](ctx, locationPolicy, loc,
    permissions.OpUpdate, permissions.OpDelete)
dto.Can = capBlock{
    Update: slices.Contains(ops, permissions.OpUpdate), // frontend: Lab shows on this
    Delete: slices.Contains(ops, permissions.OpDelete),
}
```

For this to work p44 must make its policy objects reachable from the DTO layer
(today they are constructed inline in `app.go` and handed only to
`WithPermissions`). That refactor, the `/auth/me` manifest enrichment for
row-less/nav affordances, and the frontend flag-rendering are **p44-side** and
planned separately. This document covers only the R3 primitive that makes an
honest, drift-free projection possible.

---

## Phasing summary

Superseded by the review adjustments: A + B + C shipped together as one step
(`Allow`, `AllowedOps`, `AllowResource`, scope-aware read verdict, decorator
convenience methods with `EntityID` via IDFunc, unit + scope + parity tests).
One version bump; bump the p44 pin after tagging.
