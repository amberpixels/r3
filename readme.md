# R3 — Universal Repository Abstraction

## Everything is a repo. Everything is R3.

**Write once, query anywhere.** R3 provides identical CRUD operations across in-memory slices, YAML files, JSON APIs, PostgreSQL, MongoDB, Redis — any data source. The same business logic works whether your data lives in a database or a JSON file.

Perfect for testing (use slices), prototyping (use files), and production (use databases) without changing a single line of business code. When you need raw SQL optimization, drop to `sqlc` queries seamlessly.

```go
q := r3.NewListParams().
    Where("status", "active").
    Where("age", ">", 18).
    Sort("created_at", "desc").
    Limit(10)

// This exact code works against PostgreSQL, []User, YAML files, HTTP APIs...
// repo as DB, files, slice, etc
users, total, err := repo.List(ctx, q)
```
