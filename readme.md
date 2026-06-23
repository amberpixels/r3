<p align="center">
  <img src="logo.svg" alt="R3" width="230">
</p>

<div align="center">

### Everything is a repo. Everything is an R3.

A universal, backend-agnostic CRUD repository abstraction for Go.

[![Go Reference](https://pkg.go.dev/badge/github.com/amberpixels/r3.svg)](https://pkg.go.dev/github.com/amberpixels/r3)
[![CI](https://github.com/amberpixels/r3/actions/workflows/go.yml/badge.svg)](https://github.com/amberpixels/r3/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/amberpixels/r3)](https://goreportcard.com/report/github.com/amberpixels/r3)
[![Go Version](https://img.shields.io/github/go-mod/go-version/amberpixels/r3)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-yellow.svg)](LICENSE)

</div>

---

R3 (pronounced *"ree"* /riː/, as in **repo**) provides a single generic `CRUD[T, ID]`
interface that works identically across PostgreSQL, MySQL, SQLite, MongoDB,
JSON/YAML/TOML files, and any other data source. Your business code talks to
`r3.CRUD` - it never knows or cares what's behind it.

```go
// Same interface, same query, different backends
userRepo    r3.CRUD[User, int64]      // PostgreSQL via GORM
productRepo r3.CRUD[Product, string]  // MongoDB
configRepo  r3.CRUD[Config, string]   // YAML files on disk
```

> [!NOTE]
> R3 is in **early development** (pre-1.0). The core API is stable in spirit, but
> details may change before a tagged release. Questions, ideas, and feedback are
> very welcome - see [Feedback](#feedback).

## Contents

- [Why R3](#why-r3)
- [Install](#install)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Filters](#filters)
- [Pagination](#pagination)
- [Transactions](#transactions)
- [URL Query Parsing](#url-query-parsing)
- [Requirements](#requirements)
- [AI disclosure](#ai-disclosure)
- [Feedback](#feedback)
- [License](#license)

## Why R3

R3 is **not** about swapping backends. Most systems pick a database and stick with it.

R3 is about the fact that real systems use **multiple** data sources: a relational DB
for core data, MongoDB for event logs, config files for feature flags, an external
REST API for third-party data. Without a shared interface, each one gets its own
query patterns, its own error handling, its own permission logic.

With R3, all of them speak the same language. More importantly, **features compose
across all of them**: wrap any repo with permissions, audit history, metrics, or
validation - regardless of what storage is behind it.

## Install

```bash
go get github.com/amberpixels/r3
```

Then pull in the driver(s) you need, e.g. `github.com/amberpixels/r3/drivers/gorm`.

## Quick Start

```go
import (
    "github.com/amberpixels/r3"
    r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// Define your model (standard GORM model)
type City struct {
    ID   int64  `gorm:"primaryKey"`
    Name string
}

// Create a repository
cityRepo := r3gorm.NewGormCRUD[City, int64](db)

// Create
city, err := cityRepo.Create(ctx, City{Name: "Berlin"})

// Get by ID — missing records return r3.ErrNotFound on every backend
city, err := cityRepo.Get(ctx, 42)
if errors.Is(err, r3.ErrNotFound) {
    // respond 404, etc.
}

// List with filters, sorting, and pagination.
// Short-form helpers (r3.Eq, r3.Gt, ...) keep simple filters terse.
cities, total, err := cityRepo.List(ctx, r3.Query{
    Filters: r3.Filters{
        r3.Eq("name", "Berlin"),
    },
    Sorts: r3.Sorts{
        r3.NewSortAscSpec(r3.NewFieldSpec("name")),
    },
    Pagination: r3.NewPaginationSpec(1, 25),
})

// Count matching records without materializing rows
n, err := cityRepo.Count(ctx, r3.Query{Filters: r3.Filters{r3.Eq("name", "Berlin")}})

// Update
city.Name = "Munich"
city, err = cityRepo.Update(ctx, city)

// Patch (partial update - only specified fields)
city, err = cityRepo.Patch(ctx, city, r3.Fields{r3.NewFieldSpec("name")})

// Delete
err = cityRepo.Delete(ctx, 42)
```

## Architecture

R3 is organized in five layers. Each layer has a clear responsibility and depends
only on the layers above it.

```
r3 (core)           Interfaces + query model. Zero dependencies.
  |
  +-- dialects/     Pure converters: r3 types <-> format-specific representations.
  |                 No I/O, no state. Two categories:
  |                   Data-store:    sql, bson
  |                   Serialization: json, yaml, toml, url
  |
  +-- engine/       Complete CRUD implementations per storage category.
  |                 The heavy lifting lives here.
  |                   sql   - database/sql + reflection + Flavor
  |                   mongo - MongoDB driver + reflection
  |                   file  - filesystem + codecs + in-memory query eval
  |
  +-- drivers/      Ready-to-use constructors for specific libraries.
  |                   pq, pgx, mysql, sqlite3 - wrap engine/sql
  |                   gorm, bun, gopg         - ORM-native, share query prep
  |                   mongo                   - wraps engine/mongo
  |
  +-- features/     Composable decorators that wrap ANY r3.CRUD[T, ID].
                      permissions, history, metrics, validation,
                      softdelete, transactor
```

### Core (`r3` package)

The interfaces and query model. This is the contract everything else implements.

**Interfaces:**
- `CRUD[T, ID]` - Full read+write repository (composes Querier + Commander)
- `Querier[T, ID]` - Read-only: `Get`, `List`
- `Commander[T, ID]` - Write-only: `Create`, `Update`, `Patch`, `Delete`
- `Transactor[T, ID]` - Opt-in transaction support: `BeginTx`

**Query model** - a single composable `Query` struct:
- `Filters` - Field-operator-value conditions with recursive AND/OR groups
- `Sorts` - Multi-column sort with direction and NULLS FIRST/LAST
- `PaginationSpec` - Offset-based (page number + page size)
- `CursorSpec` - Keyset/cursor-based (forward/backward with opaque tokens)
- `Fields` - Column selection (SELECT specific fields)
- `Preloads` - Eager loading of related entities

Queries are immutable values. `MergeWith()` combines queries from different sources
(e.g. defaults + user request + permission scope) without mutation.

### Dialects

Stateless, bidirectional converters between r3 types and format-specific representations.

**Data-store dialects** convert r3 queries into storage-native primitives:
- `dialects/sql` - `FilterSpec` -> `WHERE status = ? AND age > ?` with parameterized args
- `dialects/bson` - `FilterSpec` -> `bson.D{{Key: "status", Value: "active"}}`

**Serialization dialects** convert r3 queries to/from interchange formats:
- `dialects/json` - REST API request/response bodies
- `dialects/yaml` - Configuration files
- `dialects/toml` - Configuration files
- `dialects/url` - URL query parameters (`?sort=name:asc&page=2&status=active`)

Dialects are pure functions. They have no I/O, no database connections, no state.
Engines and drivers consume them; most application code doesn't import them directly.

### Engines

Complete `r3.CRUD` implementations for a **category** of storage backend.
Each engine handles reflection, query building, and execution for its storage type.

- `engine/sql` - Generic SQL via `database/sql`. Uses `Flavor` to handle
  differences between Postgres ($1 placeholders, RETURNING), MySQL (? placeholders,
  LAST_INSERT_ID), and SQLite. Provides `BaseCRUD[T, ID]` that raw SQL drivers embed,
  and `PreparedListQuery` that ORM drivers share for filter/sort/pagination translation.

- `engine/mongo` - MongoDB via the official Go driver v2. Handles BSON document
  building, projection, cursor pagination, relation preloading via separate queries.

- `engine/file` - Filesystem-based storage with pluggable codecs (JSON, YAML).
  Applies filters, sorts, and pagination in-memory. Supports single-file
  (one JSON per collection) and directory (one file per entity) modes.

### Drivers

Ready-to-use constructors that wire up an engine for a specific client library.

**Raw SQL drivers** (embed `engine/sql.BaseCRUD`):

| Driver | Package | Library | Notes |
|--------|---------|---------|-------|
| PostgreSQL | `drivers/pq` | [lib/pq](https://github.com/lib/pq) | `$1` placeholders, RETURNING |
| PostgreSQL | `drivers/pgx` | [jackc/pgx](https://github.com/jackc/pgx) | `$1` placeholders, RETURNING |
| MySQL | `drivers/mysql` | [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) | `?` placeholders, no RETURNING |
| SQLite | `drivers/sqlite3` | [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) | `?` placeholders, RETURNING (3.35+) |

**ORM drivers** (use ORM API natively, share `PreparedListQuery` for query translation):

| Driver | Package | Library | Preloads | Soft-delete |
|--------|---------|---------|----------|-------------|
| GORM | `drivers/gorm` | [gorm.io/gorm](https://github.com/go-gorm/gorm) | Preload() | Unscoped() |
| Bun | `drivers/bun` | [uptrace/bun](https://github.com/uptrace/bun) | Relation() | WhereAllWithDeleted() |
| go-pg | `drivers/gopg` | [go-pg/pg/v10](https://github.com/go-pg/pg) | Relation() | AllWithDeleted() |

**NoSQL drivers:**

| Driver | Package | Library |
|--------|---------|---------|
| MongoDB | `drivers/mongo` | [mongo-driver/v2](https://github.com/mongodb/mongo-go-driver) |

All drivers expose a `Raw()` escape hatch for queries that go beyond the r3 interface.

### Features (Decorators)

Composable middleware that wraps **any** `r3.CRUD[T, ID]`, regardless of backend.
This is where R3's "everything is a repo" philosophy pays off - the same
permission logic works for your Postgres entities and your MongoDB logs.

```go
// Stack features via decoration:
repo := permissions.WithPermissions(
    history.WithHistory(
        validation.WithValidation(
            r3gorm.NewGormCRUD[Order, int64](db),
            orderValidator,
        ),
        historyStore, history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
    ),
    orderPermissions,
)
// repo is still r3.CRUD[Order, int64] - fully transparent
```

**Available features:**

- **permissions** - Policy-based authorization. Gates every CRUD operation through a
  user-defined `Checker`. Supports entity-aware row-level checks and scope injection
  (automatic filter injection into List queries). Bring your own auth logic.

- **history** - Change tracking / audit log. Records every mutation as a `ChangeRecord`
  with field-level diffs. Supports snapshots, revert-to-version, and tree queries.
  The history store is itself an `r3.CRUD[ChangeRecord, string]`.

- **metrics** - Domain-level analytics. 10 built-in collectors (action counts, latency,
  popularity, error rates, etc.). Configurable time bucketing, aggregation, and retention.
  The metrics store is itself an `r3.CRUD[MetricRecord, string]`.

- **validation** - Pre-mutation validation. Bring your own validator
  (go-playground/validator, ozzo-validation, plain Go). Patch-aware and
  state-transition-aware (can compare new vs existing entity).

- **softdelete** - Adds `Restore()` and `HardDelete()` to any CRUD that supports soft-delete.

- **transactor** - Surfaces transaction capabilities (`BeginTx`, `InTx`) from the
  underlying driver.

## Filters

Build filters with the short-form helpers (a plain field name) for the common
case, or drop down to the `FieldSpec`-based forms when you need table hints or
nested paths:

```go
// Short-form helpers — terse, take a plain field name
r3.Eq("status", "active")
r3.Gt("age", 18)
r3.In("country", []string{"DE", "FR"})
r3.Like("name", "%john%")
r3.ILike("name", "%john%")
r3.Between("price", 10, 100)        // inclusive

// FieldSpec forms — for table hints / nested paths
r3.F(r3.NewFieldSpec("status"), "active")
r3.Fop(r3.NewFieldSpec("age"), r3.OperatorGte, 18)

// Logical groups (compose either form)
r3.And(
    r3.Eq("status", "active"),
    r3.Gte("age", 18),
)

r3.Or(
    r3.Eq("role", "admin"),
    r3.Eq("role", "moderator"),
)

// NULL checks (nil value + Eq/Ne operator)
r3.Eq("deleted_at", nil)  // IS NULL
```

**Available operators:** `Eq`, `Ne`, `Gt`, `Gte`, `Lt`, `Lte`, `In`, `NotIn`,
`Like`, `NotLike`, `ILike`, `Between`, `BetweenEx`, `BetweenExInc`, `BetweenIncEx`, `Exists`.

## Pagination

**`List` paginates by default** — with no `Pagination` set it caps results at
`r3.PageSizeDefault` (100), so a forgotten pagination never accidentally scans a
whole table. There are three ways to get more:

```go
// 1. A custom page / size, per query
r3.Query{Pagination: r3.NewPaginationSpec(1, 250)}  // page 1, 250 per page

// 2. Everything, for this one query (clears the default cap)
all, total, err := repo.List(ctx, r3.Query{Pagination: r3.Unpaginated()})

// 3. Everything by default, for this repo (global opt-out)
repo := r3gorm.NewGormCRUD[City, int64](db,
    r3.WithConfig(r3.Config{Defaults: r3.DefaultsConfig{Unpaginated: true}}),
)
// repo.List(ctx) now returns all rows; individual queries can still paginate.
```

Cursor-based pagination is the alternative to offset (requires at least one sort):

```go
r3.Query{
    Cursor: r3.NewCursorAfter(nextToken, 25),
    Sorts:  r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("created_at"))},
}
```

You can also detect truncation from the returned total without changing anything:

```go
items, total, err := repo.List(ctx)
if total > int64(len(items)) {
    // there are more rows than were returned
}
```

## Transactions

```go
err := r3.InTx(ctx, repo, func(tx r3.CRUD[Order, int64]) error {
    order, err := tx.Create(ctx, newOrder)
    if err != nil {
        return err // auto-rollback
    }
    // ... more operations within the same transaction
    return nil // auto-commit
})
```

## URL Query Parsing

Parse HTTP request parameters directly into r3 queries:

```go
import r3url "github.com/amberpixels/r3/dialects/url"

// GET /api/cities?fields=id,name&sort=name:asc&page=2&page_size=25&status=active
q, err := r3url.ParseQuery(r.URL.Query(),
    r3url.WithDjangoStyleFilters("status", "name"),
)
cities, total, err := cityRepo.List(ctx, q)
```

## Requirements

- Go 1.26+

## AI disclosure

R3's code is written with heavy AI assistance - and that's by design. But the AI
is a tool here, not the author of record:

- **Every architectural decision is made by a human.** The layering, the
  interfaces, the trade-offs - those are deliberate human choices, not whatever
  a model happened to produce.
- **Every line of code is read and reviewed by a human before it's pushed.**
  Nothing lands in this repository unread.
- **The code is written AI-first.** It's deliberately optimized to be easy for AI
  to read, grep, update, and extend - not primarily for human ergonomics. Clear,
  greppable names and consistent structure win over cleverness.

Responsibility for the code is human. 🤖🤝🧑

## Feedback

R3 isn't accepting pull requests at this stage - but **questions, ideas, bug
reports, and feedback are genuinely welcome**. Please [open an issue](https://github.com/amberpixels/r3/issues),
and see [CONTRIBUTING.md](CONTRIBUTING.md) for the why and the details. It's
MIT-licensed, so you're also free to fork and adapt it for your own work. For
security issues, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © amberpixels
