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
// # Relations
//
// Many-to-many and owned has-many relations (`r3:"rel:..."` tags) are recorded
// alongside the scalar fields, as one FieldChange per relation named after the
// snake_case Go field ("photos"), holding the full member lists before and
// after: target primary keys for a many-to-many, column values (PK, parent FK
// and created_at/updated_at left out) for an owned has-many, whose children are
// re-inserted on every save. A nil relation means "not loaded" and records
// nothing; a non-nil empty one records an explicit clear ([...] -> []). Update
// preloads the old membership of every relation the caller passed non-nil, so a
// save that writes no relation costs no extra read. Order counts only for a
// many-to-many with an `order:` column; other relations compare as multisets.
//
// Reconstruction replays relation changes like any other: a many-to-many comes
// back as target stubs carrying only the primary key, an owned has-many as
// children rebuilt from their recorded columns. RevertTo restores a relation
// only from versions that recorded it: if none up to the target did, the
// reconstructed relation is nil and the stored membership is left alone rather
// than cleared. Target rows of a many-to-many are never touched by a revert.
//
// # Retention
//
// RetentionEnforcer deletes old change records by age (MaxAge) or per-entity
// version count (MaxVersions), one-by-one via r3.CRUD Delete (slow for large
// datasets; a future BatchDelete could optimize this). RetentionEnforcer.Start
// runs a ticker-based loop whose first pass fires after the interval elapses,
// not immediately on start.
package history
