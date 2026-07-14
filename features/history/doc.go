// Package history is a transparent decorator around any r3.CRUD[T, ID] that
// records every Create, Update, Patch, and Delete as a ChangeRecord.
//
// Change records and snapshots are themselves r3 entities, persisted via any
// r3.CRUD[ChangeRecord, string] and r3.CRUD[Snapshot, string], so history can
// live in the same backend as the entities or a different one (entities in
// PostgreSQL, history in MongoDB).
//
// Key features:
//   - Field-level diffs (what changed, old value, new value)
//   - Opt-in full entity snapshots via configurable SnapshotRules
//   - Tree/nested queries (e.g. City + its Locations + their Events)
//   - Revert to any historical version (purely diff-based reconstruction)
//   - r3.Actor attribution as first-class, queryable ChangeRecord columns
//   - Context-based metadata (source, request ID, etc.)
//   - Convenience query builders (QueryForRecord, QueryForType, QueryForTree, etc.)
//   - Retention policies (age-based TTL, per-entity version limits)
//
// # Actor Integration
//
// ActorID and ActorType are resolved from r3.GetActor(ctx) on every mutation
// and stored as real columns (not in the Metadata JSON blob), so the log can be
// filtered and grouped by actor (see QueryForActor). Set the actor with
// r3.WithActor(ctx, ...), typically in auth middleware; with none set, the
// zero/SystemActor (Type "system") is recorded. MetadataFunc carries only
// surrounding context (Source, Extra), not the actor.
//
// # Retention
//
// RetentionEnforcer deletes old change records by age (MaxAge) or per-entity
// version count (MaxVersions), one-by-one via r3.CRUD Delete (slow for large
// datasets; a future BatchDelete could optimize this). RetentionEnforcer.Start
// runs a ticker-based loop whose first pass fires after the interval elapses,
// not immediately on start.
package history
