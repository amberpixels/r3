# Plan: Field value codecs (portable serializers)

Status: **Increments 1 + 2 landed (core + guards + gorm bridge + filter/cursor
encode).** Authored from the
**runwell** side - the second real R3 consumer (after p44). Motivated by a
concrete gap: there is no backend-portable way to say "store this `time.Time`
field as an integer".

## Implementation status (staged rollout - "core + gorm first")

- **Increment 1 - DONE.** Core `Codec` abstraction (`r3_codec.go`), name registry
  (`RegisterCodec`/`lookupCodec`, no `init`), built-in `unixtime`/`unixmilli`/
  `unixmicro`/`unixnano`, typed `ErrUnknownCodec` + `ErrCodecNotSupported`. Tag
  `codec:<name>` parsed into `ColumnTag.Codec`; resolved onto `Attribute.Codec` in
  `schema_derive.go` (unknown name → panic wrapping `ErrUnknownCodec`, same
  developer-error-at-derivation contract as `regexp.MustCompile`). `Attribute.Type`
  stays the **domain** type; the codec reports the stored type via `Stored()`.
  **Safety guard** (`RequireCodecSupport`) wired into every not-yet-codec-aware
  backend constructor - `engine/sql` (pq/pgx/mysql/sqlite3), `engine/mongo`,
  `engine/file`, `drivers/gorm`, `drivers/bun`, `drivers/gopg` - so a declared
  codec on an unwired backend **panics at construction** instead of silently
  storing the un-encoded value. Unit tests cover encode/decode (null/zero/pointer,
  representation tolerance, UTC-canonical, precision round-trips), tag parsing,
  schema derivation, the unknown-codec panic, and the guard firing end-to-end.
  All packages green; `just lint` = 0 issues.
- **Increment 2 - DONE (transparent gorm bridge + shared filter/cursor encode).**
  Unblocks runwell (gorm-over-sqlite). The user chose the **fully transparent**
  bridge (`r3:"codec:unixtime"` only, no gorm co-tag). **Key constraint (GORM
  v1.31.2 source):** a field's serializer scan/value closures are baked at *parse*
  time in `schema.Field.setupValuerAndSetter`/`setupNewValuePool`, capturing
  `field.Serializer` and wrapping values in GORM's **unexported** `*schema.serializer`
  - so a post-`Parse` `field.Serializer =` is a no-op and the closures cannot be
  reproduced externally. **Solution (`drivers/gorm/codec.go`):** parse a
  `reflect.StructOf` *shadow* type with identical layout whose codec'd fields carry
  a generated `gorm:"serializer:<name>"` tag (name resolved from the real parsed
  schema's `DBName`, so columns always match), then **graft the shadow field's
  `Serializer`/`ValueOf`/`Set`/`NewValuePool`/`DataType` onto the real cached
  schema field in place** - the real schema keeps its own table/model type; only
  codec'd fields gain serializer behavior. `codecSerializer` adapts `r3.Codec` →
  `gschema.SerializerInterface`; one GORM serializer registered per distinct codec
  (`r3codec_<n>`, avoiding GORM's built-in `unixtime`). `wireCodecs` runs in
  `NewGormCRUD` (guard removed). Filter/cursor encode is shared:
  `engine/sql.PrepareMergedListQuerySchema` (old `PrepareMergedListQuery` delegates
  with a zero schema = no-op) calls `r3.EncodeFilterCodecs` (scalar/In/Between,
  recurses And/Or) and `r3.EncodeCursorCodecs`; `unixTimeCodec.Encode` tolerates the
  JSON RFC3339 string form so codec'd fields work as cursor keys. Verified on
  gorm-over-sqlite (no Docker): CRUD round-trip + stored-int form, null/zero/pointer,
  filter args (Lt/Gt/Between/In with `time.Time` - the runwell `NeighborOf`
  regression), sort, Update/Patch re-encode. Note: **cursor/keyset pagination is a
  raw-SQL-engine feature not implemented by the GORM driver** (pre-existing,
  unrelated to codecs); the cursor *encoder* is unit-tested in core and wired in the
  shared prep for when raw-SQL drivers land.
- **Later** - `engine/sql` raw drivers (scan wrapper + bind), `engine/mongo`
  (write hook + read has no r3 seam today), `engine/file`, `drivers/bun`/`gopg`,
  dialect serialization. Each removes its `RequireCodecSupport` guard as it lands.

---

The flagship codec is `time.Time ↔ unix int`, but the feature is general: a
field-level, **backend-agnostic** value transform (Go value ⇄ stored value)
declared once via tag and applied uniformly across **every** engine and driver -
on read, on write, and in **filter/sort/cursor arguments**.

---

## Why (consumer motivation)

runwell stores its timestamps as `INTEGER` unix-seconds in SQLite while the
domain fields are `time.Time` (`started_at`, `notified_at`, `expires_at`).

- **There is no built-in that does `time.Time` field ↔ `int` column.** GORM's
  `serializer:unixtime` is the *inverse* mapping - an `int` Go field ↔ a
  `datetime` column (`gorm.io/gorm/schema` `UnixSecondSerializer`: `Scan` reads
  the column as `sql.NullTime` then stores `.Unix()` into an int field). Point it
  at runwell's layout and `Scan` calls `sql.NullTime.Scan(int64)` →
  `unsupported Scan, storing driver.Value type int64 into type *time.Time`.
- runwell had to hand-roll a custom GORM serializer
  (`runwell/internal/components/db/serializer.go`, tag `gorm:"serializer:unixts"`).
  **That workaround only works for the `drivers/gorm` path** - it is invisible to
  `engine/sql` (pq/pgx/mysql/sqlite3), `engine/mongo`, and `engine/file`. It also
  does nothing for **filter arguments**: runwell's `NeighborOf` had to pass a raw
  `int64` to `r3.Lt("started_at", …)` because nothing serializes a WHERE-clause
  value.
- This generalizes far beyond time: money as cents (`decimal ↔ int64`), enums as
  small ints, `uuid.UUID ↔ [16]byte`/text, compressed/encrypted blobs, a struct
  stored as a JSON string, `bool ↔ 0/1` on engines without booleans. Every one of
  these is the same shape: **one declared field, two representations, applied
  everywhere.**

R3's promise is "the same interface across every backend." A serialization
concept that lives only in one driver breaks that promise. This is the r3-side
fix for what runwell currently works around (per the "fix upstream, don't paper
over it in the consumer" rule).

---

## Goal / non-goals

**Goal (v1):**
- A first-class `Codec` abstraction in core (zero-dep), with a small registry.
- Built-in `unixtime` codec (seconds default; milli/micro/nano variants) as the
  flagship, proving the design end-to-end.
- Applied across **all** engines (`sql`, `mongo`, `file`) and ORM drivers
  (`gorm`, `bun`, `gopg`) - reads, writes, **and filter/sort/cursor args**.
- Correct for null / zero / pointer, UTC-canonical, and validated by a
  backend-parity test matrix.

**Non-goals (v1):**
- Owning DDL / migrations (r3 never has - the column type is the consumer's).
- A large built-in codec library. Ship `unixtime` + the registry hook; design so
  `boolint`, `jsonstring`, `uuidbytes` are trivial follow-ups.
- Computed/virtual attributes (already reserved; out of scope here).

---

## Conventions to follow

Mirror the **existing extension patterns**, do not touch the core CRUD interfaces:

1. **Opt-in, tag-declared, per-attribute** - like `enum:` / `readonly` in
   `internal/tag`. A codec is a property of an `Attribute`, resolved during
   `SchemaOf`. No change to `Querier`/`Commander`.
2. **Registry + package helper** - like `Aggregator`/`AggregateOf`: a
   `RegisterCodec(name, Codec)` registry, built-ins registered at declaration
   (no `init`, the linter forbids it). Unknown codec name → typed error at
   `SchemaOf` time, not a silent no-op.
3. **Backend parity is mandatory, not optional** - a value codec is pure Go
   value⇄value, so *every* engine can and must apply it. Unlike Upsert/BulkPatch
   (which may degrade to a `…NotSupported` sentinel), a codec that a backend
   silently ignores is a **data-corruption bug**. If a driver genuinely can't
   wire it per-field, it must apply the codec in query-prep + post-scan rather
   than skip it. See `docs/backend-parity.md`.

---

## Design

### 1. Core - the `Codec` abstraction (`r3_codec.go`, new)

Operates on plain Go values so it is backend-neutral: each engine feeds it
whatever it scanned (SQL `int64`, bson `int32/int64`, JSON `float64`,
`[]byte`/`string`) and takes back a value to bind/marshal.

```go
// Codec transforms a Go field value to/from its stored representation.
// Implementations must be pure and stateless (they are shared + cached).
type Codec interface {
    // Encode converts a Go field value into the value handed to the backend
    // (bind param, bson value, JSON/YAML/TOML node). Return (nil, nil) for NULL.
    Encode(goValue any) (stored any, err error)

    // Decode converts a value read from any backend back into the Go field
    // value of type target. Must tolerate the representation variance across
    // backends (int64/int32/float64/[]byte/string) for the same logical type.
    Decode(stored any, target reflect.Type) (goValue any, err error)

    // Stored reports the logical stored DataType (Int, String, Bytes, ...),
    // used for dialect binding hints and cursor encoding.
    Stored() DataType
}

func RegisterCodec(name string, c Codec) // built-ins registered at declaration
func lookupCodec(name string) (Codec, bool)
```

Decision - **`Attribute.Type` stays the *domain/logical* type (e.g. `Time`)**,
not the stored type. Rationale: callers filter with domain values
(`r3.Lt("started_at", someTime)`) and frontends want a date widget; validation
should accept a `time.Time`. The codec bridges to storage at the engine/dialect
boundary. Add `Attribute.Codec Codec` (nil = identity). `Stored()` is only a
hint for binding/cursor encoding.

### 2. Tag surface (`internal/tag/tag.go`, `schema_derive.go`)

- New `ColumnTag.Codec string`, parsed from `r3:"...,codec:unixtime"` (colon
  form, consistent with `enum:`). Precision as distinct names -
  `unixtime`|`unixmilli`|`unixmicro`|`unixnano` - is simpler than parsing an arg;
  ship `unixtime` (seconds) first.
- Also honor a codec named in `db:"..."`; do **not** auto-read `gorm:"serializer:"`
  (different semantics - the inverse trap above). r3's `codec` is the source of
  truth; the gorm/bun drivers bridge to their native mechanisms (§4).
- `schema_derive.go`: resolve the name via `lookupCodec`; unknown → error. Attach
  `Codec` to the `Attribute`. Keep `inferType` reporting the domain type.

### 3. Built-in codec: `unixtime` (+ milli/micro/nano)

`time.Time`/`*time.Time` ⇄ `int64`. Rules (match runwell's proven `unixts`):
- zero `time.Time` → NULL; nil `*time.Time` → NULL; NULL → zero/nil on decode.
- `Encode` → `int64` at the configured precision, **UTC**.
- `Decode` tolerant of `int64/int/uint/float64` and `[]byte`/`string`
  (RFC3339 fallback), so it survives whatever a given backend returns.

### 4. Per-engine + per-driver application - **must cover all backends**

- **`engine/sql`** (→ pq, pgx, mysql, sqlite3): hook the scan/bind path in
  `engine/sql/reflect.go` (+ `crud.go`, `list_query.go`, `upsert.go`,
  `bulkpatch.go`, `aggregate.go`). On bind → `Codec.Encode`; on scan →
  `Codec.Decode` into the field. This is the raw `database/sql` path r3 fully
  controls; sqlite3 driver is the runwell target.
- **`engine/mongo`**: `engine/mongo/reflect.go` - apply on bson marshal/unmarshal
  (build + decode).
- **`engine/file`**: `engine/file/reflect.go` + `codec.go` - apply when encoding
  to / decoding from json/yaml/toml **and** in the in-memory query eval
  (`engine/file/filter.go`, `sort.go`) so file-backend filters compare stored
  form.
- **`drivers/gorm`** (the subtle one - GORM owns scan/bind): bridge each
  codec'd attribute to a generated GORM `schema.SerializerInterface` that
  delegates to the r3 `Codec`, and register/attach it (per-field
  `serializer:<generated>` or a `StatementModifier`) in `gormcrud.go`. Result:
  users declare `r3:"codec:unixtime"` and the gorm driver wires GORM
  transparently - **this is exactly what replaces runwell's hand-rolled
  `unixts`.**
- **`drivers/bun`, `drivers/gopg`**: prefer real per-field support (bun custom
  types / scan+append hooks). If per-field isn't clean, apply the codec in the
  driver's query-prep (encode bind + filter args) and a post-scan decode pass -
  **must not** silently skip (§Conventions.3).

### 5. Cross-cutting correctness (where runwell's `NeighborOf` bug lives)

- **Filter args** - the crux. When a filter targets a codec'd attribute, r3 must
  `Encode` the argument to stored form *before* it reaches WHERE (GORM/bun/
  `database/sql` do not serialize WHERE args via field serializers). Apply when a
  `FilterSpec` is resolved against the schema (canonical/query-prep or per-dialect
  `dialects/*/filter.go`). Handle scalar, `In` slices, and `Between` pairs. This
  is the exact gap that forced runwell to pass a raw `int64`.
- **Sort / cursor** - keyset pagination encodes field values into the cursor
  (`r3_cursor.go`, `dialects/*/cursor.go`). A codec'd field must Encode/Decode
  symmetrically or cursors corrupt.
- **Aggregation** - min/max/group-by on a codec'd field: group keys in stored
  form, Decode results (`engine/*/aggregate.go`). Done via the shared core helper
  `r3.DecodeAggregateCodecs` (decodes codec'd group-by columns and `MIN`/`MAX`;
  `SUM`/`AVG`/`COUNT` stay raw), wired into GORM; other backends wire it as their
  codec support lands. See [`plan-aggregate-codec-decode.md`](./plan-aggregate-codec-decode.md).
- **Schema validation** - `schema_validate.go`: a `time.Time` arg against a
  `unixtime` field must validate (domain type), not be rejected for "type
  mismatch" against an int column.
- **Null/zero/pointer** - one behavior across every engine.

### 6. Work items (checklist)

Core
- [x] `r3_codec.go`: `Codec` iface, registry (`RegisterCodec`/`lookupCodec`),
      built-in `unixtime` (+ milli/micro/nano), typed `ErrUnknownCodec`,
      `ErrCodecNotSupported`, `RequireCodecSupport` guard helper.
- [x] `internal/tag/tag.go`: parse `codec:<name>` into `ColumnTag.Codec`.
- [x] `schema.go` / `schema_derive.go`: `Attribute.Codec`; resolve name (unknown →
      panic wrapping `ErrUnknownCodec`).
- [x] Guard every unwired backend constructor with `RequireCodecSupport` (engine/sql,
      engine/mongo, engine/file, drivers/gorm, drivers/bun, drivers/gopg).

Engines
- [ ] `engine/sql/reflect.go` (+ crud/list_query/upsert/bulkpatch/aggregate): bind+scan.
- [ ] `engine/mongo/reflect.go`: marshal+unmarshal.
- [ ] `engine/file/reflect.go` + `codec.go` (+ filter/sort eval): encode+decode.

Query prep (cross-cutting)
- [x] Filter-arg encode for codec'd fields (scalar/In/Between, recurses And/Or) -
      `r3.EncodeFilterCodecs`, applied in `engine/sql.PrepareMergedListQuerySchema`.
- [x] Cursor encode for codec'd sort keys - `r3.EncodeCursorCodecs` (Encode tolerates
      the JSON RFC3339 form). Wired in the shared prep; consumed by raw-SQL cursor
      pagination once those drivers land (GORM driver has no cursor pagination).

Drivers
- [x] `drivers/gorm`: transparent auto-bridge r3 `Codec` → generated GORM serializer
      via shadow-parse + graft (`drivers/gorm/codec.go`).
- [ ] `drivers/bun`, `drivers/gopg`: per-field bridge, else query-prep + post-scan.

Tests (extend `docs/backend-parity.md` matrix)
- [ ] Round-trip a model with a `unixtime` field through CRUD + `Lt/Gt/Between/In`
      + sort + cursor pagination + aggregate(min/max) on: sqlite3, pq, pgx, mysql,
      mongo (testcontainers), file(json/yaml/toml), gorm, bun. Assert the Go value
      round-trips **and** the stored form is an int.
- [ ] Null / zero-time / nil-pointer across every backend.
- [ ] Regression: **filter argument on a codec'd field is encoded** (the runwell
      `NeighborOf` case) - a `time.Time` bound produces an int comparison.

Docs
- [ ] readme "Serialization" section + `docs/schema-design.md`; call out the
      distinction from GORM's inverse `unixtime` prominently.

Downstream adoption (runwell - do after r3 tags a release)
- [ ] Delete `internal/components/db/serializer.go`; drop the `registerUnixTimeSerializer`
      call in `Connect`.
- [ ] Entity tags `gorm:"serializer:unixts"` → `r3:"...,codec:unixtime"` on
      `activity.StartedAt`/`NotifiedAt`, `auth.StravaAuth.ExpiresAt`.
- [ ] `NeighborOf` can pass a `time.Time` bound (r3 encodes it); bump the r3 pin.

---

## Open questions / decisions for the r3 agent

- **Naming:** `codec` vs `serializer` vs `transform`. Recommend **`codec`** - avoids
  colliding with GORM's `serializer` (whose inverse semantics are the trap here).
- **Precision syntax:** distinct names (`unixmilli`) vs an arg (`unixtime:milli`).
  Recommend distinct names (no arg-parsing).
- **`Attribute.Type`:** keep the **domain** type (recommended, §1) vs switch to the
  stored type. Domain keeps caller-arg validation + filter widgets sane.
- **Registry visibility in v1:** public `RegisterCodec` now, or built-ins only with
  the hook unexported until a consumer needs custom codecs.
- **bun/gopg:** confirm per-field feasibility; otherwise the query-prep + post-scan
  fallback is acceptable (but never silent-skip).

## References

- Consumer origin (the workaround this replaces): `runwell/internal/components/db/serializer.go`
  (`unixts`), entities in `runwell/internal/business/*` (`gorm:"serializer:unixts"`),
  and the `NeighborOf` filter-arg gap in `runwell/internal/components/db/db.go`.
- The inverse built-in: `gorm.io/gorm/schema` `UnixSecondSerializer` (`unixtime`).
- Existing extension precedents to mirror: `r3_aggregate.go` + `AggregateOf`
  (opt-in capability), `internal/tag` `enum:`/`readonly` (tag-declared attribute
  property), `docs/plan-upsert-and-bulk-update.md` (this plan's format + parity rules).
