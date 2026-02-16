// Package history provides activity log / change tracking for r3 CRUD repositories.
//
// It works as a decorator around any r3.CRUD[T, ID] implementation, recording every
// Create, Update, Patch, and Delete as a ChangeRecord. The decorator is transparent:
// the wrapped repository still satisfies r3.CRUD[T, ID].
//
// "Everything is a R3po" — change records and snapshots are themselves r3 entities,
// stored via any r3.CRUD[ChangeRecord, string] and r3.CRUD[Snapshot, string].
// The history feature has zero knowledge of storage backends. You can use the
// exact same driver (SQL, GORM, MongoDB, etc.) that you use for your entities,
// or a completely different one (e.g. entities in PostgreSQL, history in MongoDB).
//
// Key features:
//   - Field-level diffs (what changed, old value, new value)
//   - Opt-in full entity snapshots via configurable SnapshotRules
//   - Tree/nested queries (e.g. Campaign + its Adsets + their Creatives)
//   - Revert to any historical version (purely diff-based reconstruction)
//   - Context-based metadata (actor, source, request ID, etc.)
//   - Convenience query builders (QueryForRecord, QueryForType, QueryForTree, etc.)
package history
