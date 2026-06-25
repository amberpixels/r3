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
//   - r3.Actor integration for automatic actor attribution — ActorID/ActorType
//     are first-class, queryable columns on ChangeRecord (filter the log by actor)
//   - Context-based metadata (source, request ID, etc.)
//   - Convenience query builders (QueryForRecord, QueryForType, QueryForTree, etc.)
//   - Retention policies (age-based TTL, per-entity version limits)
//
// # Actor Integration
//
// The history decorator records the actor as first-class ChangeRecord fields —
// ActorID and ActorType — resolved from r3.GetActor(ctx) on every mutation.
// Because they are real columns (not buried in the Metadata JSON blob), the
// activity log can be filtered and grouped by actor (see QueryForActor).
//
// To attribute a change to a specific actor, set it on the context with
// r3.WithActor(ctx, r3.Actor{ID: "42", Type: "user"}) — typically in auth
// middleware. When no actor is set, the zero/SystemActor (Type "system") is
// recorded. MetadataFunc is for surrounding context only (Source, Extra); it no
// longer carries the actor.
//
// # Retention
//
// RetentionEnforcer deletes old change records based on age (MaxAge) or
// per-entity version count (MaxVersions). It deletes records one-by-one via
// the r3.CRUD Delete method. For large datasets this may be slow; a future
// BatchDelete interface could optimize bulk deletions for capable backends.
//
// RetentionEnforcer.Start runs periodic enforcement via a ticker-based
// background loop. The first enforcement fires after the configured interval
// elapses, not immediately on start.
package history
