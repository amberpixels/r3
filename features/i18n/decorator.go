package i18n

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/amberpixels/r3"
)

// CRUD localizes reads (Get/List overlay translated values for the r3.GetLocale
// locale) and tracks staleness on writes (Update/Patch mark translations of
// changed source fields stale). The translation store is any
// r3.CRUD[Translation, string]. Wrap read-facing repos only - see the package
// doc's read-modify-write warning.
type CRUD[T any, ID comparable] struct {
	inner  r3.CRUD[T, ID]
	store  r3.CRUD[Translation, string]
	opts   Options[T, ID]
	fields map[string]int // storage field name -> struct field index on T
}

var _ r3.CRUD[struct{}, int64] = &CRUD[struct{}, int64]{}
var _ r3.Aggregator = &CRUD[struct{}, int64]{}
var _ r3.RelationAggregator = &CRUD[struct{}, int64]{}

// WithTranslations wraps inner with locale-aware reads and staleness tracking.
// IDFunc and Fields are required; it panics on misconfiguration (unknown or
// non-string field, missing IDFunc) so wiring errors fail at startup, not at
// request time.
func WithTranslations[T any, ID comparable](
	inner r3.CRUD[T, ID],
	store r3.CRUD[Translation, string],
	optFns ...Option[T, ID],
) *CRUD[T, ID] {
	var opts Options[T, ID]
	for _, fn := range optFns {
		fn(&opts)
	}
	applyDefaults(&opts)

	if opts.IDFunc == nil {
		panic("i18n: WithIDFunc is required")
	}
	if len(opts.Fields) == 0 {
		panic("i18n: WithFields is required")
	}
	fields, err := resolveFields[T](opts.Fields)
	if err != nil {
		panic(err)
	}

	return &CRUD[T, ID]{inner: inner, store: store, opts: opts, fields: fields}
}

// Inner returns the underlying CRUD repository (unwrapped).
func (c *CRUD[T, ID]) Inner() r3.CRUD[T, ID] { return c.inner }

// Unwrap returns the wrapped CRUD so decorator-chain walks (capability
// detection, transaction propagation) can reach the backend.
func (c *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] { return c.inner }

// Rewrap rebuilds this decorator around inner, re-applying the i18n layer on top
// of a transaction-bound CRUD.
func (c *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner, store: c.store, opts: c.opts, fields: c.fields}
}

// Translations returns the translation store for querying or writing rows
// directly (workers, admin editors) - use with the Query* builders and Upsert.
func (c *CRUD[T, ID]) Translations() r3.CRUD[Translation, string] { return c.store }

// Get retrieves an entity and overlays translations for the context locale.
func (c *CRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	entity, err := c.inner.Get(ctx, id, qarg...)
	if err != nil {
		return entity, err
	}
	c.overlay(ctx, []T{}, &entity)
	return entity, nil
}

// List retrieves entities and overlays translations for the context locale,
// fetching the whole page's translations in a single store query.
func (c *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	items, total, err := c.inner.List(ctx, qarg...)
	if err != nil {
		return items, total, err
	}
	c.overlay(ctx, items, nil)
	return items, total, nil
}

// Count returns the number of matching entities. Nothing to localize.
func (c *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	return c.inner.Count(ctx, qarg...)
}

// Aggregate passes through: aggregated rows carry counts/sums, not translatable
// text, so nothing is localized.
func (c *CRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	agg, ok := c.inner.(r3.Aggregator)
	if !ok {
		return nil, r3.ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, qarg...)
}

// AggregateThroughRelation passes through: aggregated values are counts/sums,
// not translatable text.
func (c *CRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	agg, ok := c.inner.(r3.RelationAggregator)
	if !ok {
		return nil, r3.ErrRelationAggregateNotSupported
	}
	return agg.AggregateThroughRelation(ctx, relation, qarg...)
}

// Create passes through: a new entity has no translations yet.
func (c *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	return c.inner.Create(ctx, entity)
}

// Update modifies an entity, then marks translations whose source text
// changed as stale (best-effort; the update itself never fails because of it).
func (c *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	result, err := c.inner.Update(ctx, entity)
	if err != nil {
		return result, err
	}
	c.markStale(ctx, result)
	return result, nil
}

// Patch partially updates an entity, then marks translations whose source
// text changed as stale (best-effort).
func (c *CRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	result, err := c.inner.Patch(ctx, entity, fields)
	if err != nil {
		return result, err
	}
	c.markStale(ctx, result)
	return result, nil
}

// Delete removes an entity; with WithDeleteOnEntityDelete it also removes the
// entity's translations (best-effort).
func (c *CRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	if err := c.inner.Delete(ctx, id); err != nil {
		return err
	}
	if c.opts.DeleteWithEntity {
		c.deleteTranslations(ctx, fmt.Sprint(id))
	}
	return nil
}

// overlay localizes a slice (items) or one entity (single != nil) for the context
// locale. Best-effort: on a store failure the entities are left untranslated.
func (c *CRUD[T, ID]) overlay(ctx context.Context, items []T, single *T) {
	if c.opts.SkipOverlay {
		return
	}
	locale := r3.GetLocale(ctx)
	if locale == "" {
		return
	}

	var q r3.Query
	switch {
	case single != nil:
		q = QueryFor(c.opts.EntityType, c.entityID(*single), locale)
	case len(items) > 0:
		ids := make([]string, len(items))
		for i := range items {
			ids[i] = c.entityID(items[i])
		}
		q = QueryForBatch(c.opts.EntityType, locale, ids)
	default:
		return
	}

	translations, _, err := c.store.List(ctx, q)
	if err != nil {
		c.handleError(ctx, fmt.Errorf("i18n: overlay read for %s failed: %w", c.opts.EntityType, err))
		return
	}
	if len(translations) == 0 {
		return
	}

	// entity_id -> field -> value, skipping empties and (optionally) stale.
	values := make(map[string]map[string]string)
	for _, tr := range translations {
		if tr.Value == "" || (tr.Stale && c.opts.ExcludeStale) {
			continue
		}
		if _, ok := c.fields[tr.Field]; !ok {
			continue // a field this decorator does not manage
		}
		m := values[tr.EntityID]
		if m == nil {
			m = make(map[string]string, len(c.fields))
			values[tr.EntityID] = m
		}
		m[tr.Field] = tr.Value
	}

	apply := func(e *T) {
		for field, val := range values[c.entityID(*e)] {
			setFieldValue(e, c.fields[field], val)
		}
	}
	if single != nil {
		apply(single)
		return
	}
	for i := range items {
		apply(&items[i])
	}
}

// markStale patches Stale=true on translations whose SourceHash no longer matches
// the entity's current source text. Best-effort: failures go to the error handler,
// never to the caller.
func (c *CRUD[T, ID]) markStale(ctx context.Context, entity T) {
	translations, _, err := c.store.List(ctx, QueryForEntity(c.opts.EntityType, c.entityID(entity)))
	if err != nil {
		c.handleError(ctx, fmt.Errorf("i18n: staleness read for %s failed: %w", c.opts.EntityType, err))
		return
	}

	staleField := r3.Fields{r3.NewFieldSpec("stale")}
	for _, tr := range translations {
		idx, ok := c.fields[tr.Field]
		if !ok || tr.Stale {
			continue
		}
		if Hash(fieldValue(entity, idx)) == tr.SourceHash {
			continue
		}
		tr.Stale = true
		if _, err := c.store.Patch(ctx, tr, staleField); err != nil {
			c.handleError(ctx, fmt.Errorf("i18n: marking translation %s stale failed: %w", tr.ID, err))
		}
	}
}

// deleteTranslations removes every translation row of an entity (best-effort).
func (c *CRUD[T, ID]) deleteTranslations(ctx context.Context, entityID string) {
	translations, _, err := c.store.List(ctx, QueryForEntity(c.opts.EntityType, entityID))
	if err != nil {
		c.handleError(ctx, fmt.Errorf("i18n: cleanup read for %s/%s failed: %w", c.opts.EntityType, entityID, err))
		return
	}
	for _, tr := range translations {
		if err := c.store.Delete(ctx, tr.ID); err != nil {
			c.handleError(ctx, fmt.Errorf("i18n: deleting translation %s failed: %w", tr.ID, err))
		}
	}
}

func (c *CRUD[T, ID]) entityID(entity T) string {
	return fmt.Sprint(c.opts.IDFunc(entity))
}

// handleError reports a failed best-effort side operation via the configured
// ErrorHandler, or slog when none is set.
func (c *CRUD[T, ID]) handleError(ctx context.Context, err error) {
	if c.opts.ErrorHandler != nil {
		c.opts.ErrorHandler(err)
		return
	}
	slog.ErrorContext(ctx, "r3i18n: side operation failed", "entity_type", c.opts.EntityType, "error", err)
}

// typeName returns the bare struct name of T (e.g. "Location").
func typeName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		return "unknown"
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}
