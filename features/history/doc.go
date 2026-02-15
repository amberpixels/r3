// Package history provides activity log / change tracking for r3 CRUD repositories.
//
// It works as a decorator around any r3.CRUD[T, ID] implementation, recording every
// Create, Update, Patch, and Delete as a ChangeRecord. The decorator is transparent:
// the wrapped repository still satisfies r3.CRUD[T, ID].
//
// Change records are stored via the Store interface, which is backend-agnostic.
// You can store history in the same database as your entities, in a separate database,
// or even in a completely different storage system (e.g. entities in PostgreSQL,
// history in MongoDB).
//
// Key features:
//   - Field-level diffs (what changed, old value, new value)
//   - Full entity snapshots for instant revert to any version
//   - Tree/nested queries (e.g. Campaign + its Adsets + their Creatives)
//   - Revert to any historical version
//   - Context-based metadata (actor, source, request ID, etc.)
package history
