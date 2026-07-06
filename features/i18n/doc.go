// Package i18n provides entity-content translations for r3 CRUD repositories.
//
// It works as a decorator around any r3.CRUD[T, ID]: on reads (Get, List) it
// overlays translated values onto the entity's translatable fields for the
// locale carried in the context (r3.WithLocale / r3.GetLocale); on writes
// (Update, Patch) it detects source-text changes and marks the affected
// translations stale, so a translation worker knows what to redo. The
// decorator is transparent: the wrapped repository still satisfies
// r3.CRUD[T, ID].
//
// "Everything is a R3po" — translations are themselves r3 entities, stored
// via any r3.CRUD[Translation, string]. The feature has zero knowledge of
// storage backends: entities can live in PostgreSQL and translations in
// SQLite, MongoDB, or anywhere else.
//
// Key properties:
//   - Locale from context: r3.WithLocale(ctx, "ru") in middleware is the only
//     per-request wiring; every read through the decorator localizes itself.
//   - Batched overlay: List fetches all translations for the returned page in
//     ONE store query (entity_id IN (...)), never N+1.
//   - Fallback to source: a field with no (or empty) translation keeps the
//     entity's original text.
//   - Staleness via SourceHash: each Translation records a hash of the source
//     text it translated. When Update/Patch changes a source field, matching
//     translations are patched Stale=true (best-effort; mutations never fail
//     because of it). Stale translations are still served by default — an
//     outdated translation usually beats the wrong language — pass
//     WithoutStale to hide them.
//   - Delete: entity deletion leaves translations in place by default (safe
//     with soft-delete/restore); opt into cleanup with WithDeleteOnEntityDelete.
//
// # Read-modify-write warning
//
// Wrap READ-facing repositories only. If an admin/editor flow Gets an entity
// through this decorator with a locale set and saves it back, the translated
// values would be written over the source text. Keep mutation paths on the
// unwrapped repository (or ensure their contexts carry no locale).
//
// Usage:
//
//	repo := i18n.WithTranslations[Location, int64](
//	    inner,
//	    translationStore, // any r3.CRUD[i18n.Translation, string]
//	    i18n.WithIDFunc[Location, int64](func(l Location) int64 { return l.ID }),
//	    i18n.WithFields[Location, int64]("title", "description"),
//	)
//
//	// Serving a Russian-speaking user:
//	ctx = r3.WithLocale(ctx, "ru")
//	loc, _ := repo.Get(ctx, 42) // Title/Description come back in Russian when translated
//
// Translation writers (background workers, admin editors) use the same store
// directly: compute SourceHash with Hash(sourceText), then Upsert. The query
// builders (QueryFor, QueryForEntity, QueryStale) keep the worker's queue
// logic in one place.
package i18n
