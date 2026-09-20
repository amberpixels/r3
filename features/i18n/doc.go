// Package i18n adds entity-content translations to any r3.CRUD[T, ID]. The
// decorator wraps the repo and transparently satisfies it: on reads (Get, List)
// it overlays translated values onto translatable fields for the context locale
// (r3.WithLocale / r3.GetLocale); on writes (Update, Patch) it detects source-text
// changes and marks the affected translations stale for a worker to redo.
//
// "Everything is a R3po" - translations are themselves r3 entities, stored via any
// r3.CRUD[Translation, string]. Entities can live in PostgreSQL and translations
// in SQLite, MongoDB, or anywhere else.
//
// Key properties:
//   - Locale from context: r3.WithLocale(ctx, "ru") in middleware is the only
//     per-request wiring; every read localizes itself.
//   - Batched overlay: List fetches a page's translations in ONE store query
//     (entity_id IN (...)), never N+1. A declared preloaded relation adds one
//     more query for the whole page, not one per child.
//   - Preloaded children: by default only the decorated entity is translated, so
//     a preloaded child keeps its source language. Declare the relations that
//     carry translatable text with WithPreloads (see below).
//   - Fallback to source: a field with no (or empty) translation keeps its
//     original text.
//   - Staleness via SourceHash: each Translation stores a hash of the source text
//     it translated. When Update/Patch changes a source field, matching
//     translations are patched Stale=true (best-effort; mutations never fail over
//     it). Stale translations are still served by default (an outdated translation
//     usually beats the wrong language) - pass WithoutStale to hide them.
//   - Delete: leaves translations in place by default (safe with
//     soft-delete/restore); opt into cleanup with WithDeleteOnEntityDelete.
//
// # Read-modify-write warning
//
// Wrap READ-facing repositories only. If an admin/editor flow Gets an entity
// through this decorator with a locale set and saves it back, the translated
// values overwrite the source text. Keep mutation paths on the unwrapped
// repository (or ensure their contexts carry no locale).
//
// WithPreloads widens this from one row to a row plus everything preloaded onto
// it: saving a parent read under a locale can persist translated child text
// through whatever association sync the driver performs. With preloads declared,
// "read-facing only" stops being cautious advice and becomes load-bearing.
//
// # Preloaded children
//
// Preloads are resolved by the driver, below every decorator, so wrapping the
// child's own repo in i18n does nothing for a parent's preload - that repo is
// never consulted. Declare the relation on the parent instead:
//
//	repo := i18n.WithTranslations[Location, int64](
//	    inner, translationStore,
//	    i18n.WithIDFunc[Location, int64](func(l Location) int64 { return l.ID }),
//	    i18n.WithFields[Location, int64]("title", "description"),
//	    i18n.WithPreloads[Location, int64](
//	        i18n.TranslatedRelation("City", func(c City) int64 { return c.ID }, "name"),
//	        i18n.TranslatedRelation("Events", func(e Event) int64 { return e.ID }, "name"),
//	    ),
//	)
//
//	loc, _ := repo.Get(r3.WithLocale(ctx, "ru"), 42) // loc.City.Name is Russian too
//
// The relation field may be C, *C, []C or []*C, and is named by its Go field
// name, the way r3.Preloads names it. Children nest one level: a child's own
// children are not reached.
//
// This is a read overlay only. A parent's Update/Patch never marks a child's
// translations stale; that belongs to whatever repo owns the child.
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
// Translation writers (workers, admin editors) use the same store directly:
// compute SourceHash with Hash(sourceText), then Upsert. The query builders
// (QueryFor, QueryForEntity, QueryStale) keep the worker's queue logic in one place.
package i18n
