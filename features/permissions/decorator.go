package permissions

import (
	"context"

	"github.com/amberpixels/r3"
)

// CRUD gates every operation through the Checker before delegating to the inner
// repo. It is stateless - it stores no roles or permissions, only asks "is this
// allowed?". See the package doc for usage.
type CRUD[T any, ID comparable] struct {
	inner   r3.CRUD[T, ID]
	checker Checker[T, ID]
	opts    Options[T, ID]
}

var (
	_ r3.CRUD[any, any]     = &CRUD[any, any]{}
	_ r3.Aggregator         = &CRUD[any, any]{}
	_ r3.RelationAggregator = &CRUD[any, any]{}
)

// WithPermissions wraps inner so every operation is gated by checker.
func WithPermissions[T any, ID comparable](
	inner r3.CRUD[T, ID],
	checker Checker[T, ID],
	optFns ...Option[T, ID],
) *CRUD[T, ID] {
	var opts Options[T, ID]
	for _, fn := range optFns {
		fn(&opts)
	}
	applyDefaults(&opts)

	return &CRUD[T, ID]{
		inner:   inner,
		checker: checker,
		opts:    opts,
	}
}

// Inner returns the underlying CRUD, unwrapped - bypasses permission checking.
func (p *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return p.inner
}

// Unwrap returns the wrapped CRUD so decorator-chain walks (capability
// detection, transaction propagation) can reach the backend.
func (p *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return p.inner
}

// Rewrap rebuilds this decorator around inner, re-applying the permission layer
// on top of a transaction-bound CRUD.
func (p *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner, checker: p.checker, opts: p.opts}
}

// Create checks OpCreate permission, then delegates to inner.Create.
func (p *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	actor := r3.GetActor(ctx)
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpCreate,
		Actor:     actor,
		Entity:    &entity,
	}); err != nil {
		var zero T
		return zero, err
	}

	return p.inner.Create(ctx, entity)
}

// Get fetches first (the entity is needed for the row-level check), then checks
// OpRead - a denied entity is never leaked.
//
// If the checker is a Scoper, the fetched entity is verified against the scope
// filters, consistent with List/Count. An out-of-scope entity is reported as
// [r3.ErrNotFound] (invisible), exactly as List would omit it. Scope filters are
// evaluated in memory; an unevaluable filter fails closed (out of scope).
func (p *CRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	result, err := p.inner.Get(ctx, id, qarg...)
	if err != nil {
		return result, err
	}

	actor := r3.GetActor(ctx)
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpRead,
		Actor:     actor,
		Entity:    &result,
		EntityID:  &id,
	}); err != nil {
		var zero T
		return zero, err
	}

	// Row-level scope: the entity must satisfy the same filters List/Count apply.
	if scoper, ok := p.checker.(Scoper[T, ID]); ok {
		filters, err := scoper.Scope(ctx, actor)
		if err != nil {
			var zero T
			return zero, err
		}
		if len(filters) > 0 {
			inScope, matchErr := p.entityInScope(ctx, id, &result, filters)
			if matchErr != nil || !inScope {
				var zero T
				return zero, r3.ErrNotFound
			}
		}
	}

	return result, nil
}

// entityInScope reports whether the already-fetched entity satisfies the actor's
// scope filters. Plain column filters match in memory (no extra query). A
// relationship ("has") filter can't be evaluated in memory, so scope is verified
// with a query applying the same filters List/Count use - one extra query
// (bounded by the in-scope row count; a future native-EXISTS path can pin by PK).
func (p *CRUD[T, ID]) entityInScope(ctx context.Context, id ID, result *T, filters r3.Filters) (bool, error) {
	if !containsRelationFilter(filters) {
		return entityMatchesFilters(result, filters)
	}
	if p.opts.IDFunc == nil {
		// Can't identify the row to verify it via query; fail closed.
		return false, nil
	}
	items, _, err := p.inner.List(ctx, r3.Query{Filters: filters, Pagination: r3.Unpaginated()})
	if err != nil {
		return false, err
	}
	for i := range items {
		if p.opts.IDFunc(items[i]) == id {
			return true, nil
		}
	}
	return false, nil
}

// containsRelationFilter reports whether any filter in the tree is a relationship
// ("has") filter, which must be evaluated by the database rather than in memory.
func containsRelationFilter(filters r3.Filters) bool {
	for _, f := range filters {
		if f == nil {
			continue
		}
		if f.Relation != "" {
			return true
		}
		if containsRelationFilter(f.And) || containsRelationFilter(f.Or) {
			return true
		}
	}
	return false
}

// List checks resource-level OpRead, injects the Scoper's filters if any, then
// delegates to inner.List.
func (p *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	actor := r3.GetActor(ctx)

	// Resource-level: may this actor list this type at all?
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpRead,
		Actor:     actor,
	}); err != nil {
		return nil, 0, err
	}

	if scoper, ok := p.checker.(Scoper[T, ID]); ok {
		filters, err := scoper.Scope(ctx, actor)
		if err != nil {
			return nil, 0, err
		}
		if len(filters) > 0 {
			scopeQuery := r3.Query{Filters: filters}
			if len(qarg) > 0 {
				qarg[0] = qarg[0].MergeWith(scopeQuery)
			} else {
				qarg = append(qarg, scopeQuery)
			}
		}
	}

	return p.inner.List(ctx, qarg...)
}

// Count checks resource-level OpRead, injects the Scoper's filters if any, then
// delegates to inner.Count.
func (p *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	actor := r3.GetActor(ctx)

	// Resource-level: may this actor read this type at all?
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpRead,
		Actor:     actor,
	}); err != nil {
		return 0, err
	}

	if scoper, ok := p.checker.(Scoper[T, ID]); ok {
		filters, err := scoper.Scope(ctx, actor)
		if err != nil {
			return 0, err
		}
		if len(filters) > 0 {
			scopeQuery := r3.Query{Filters: filters}
			if len(qarg) > 0 {
				qarg[0] = qarg[0].MergeWith(scopeQuery)
			} else {
				qarg = append(qarg, scopeQuery)
			}
		}
	}

	return p.inner.Count(ctx, qarg...)
}

// Aggregate checks resource-level OpRead and injects the Scoper's filters if any
// - so grouped results cover only rows the actor may see - then delegates to the
// inner repo. Yields r3.ErrAggregateNotSupported when it has no aggregation.
func (p *CRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	agg, ok := p.inner.(r3.Aggregator)
	if !ok {
		return nil, r3.ErrAggregateNotSupported
	}

	actor := r3.GetActor(ctx)

	// Resource-level: may this actor read this type at all?
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpRead,
		Actor:     actor,
	}); err != nil {
		return nil, err
	}

	if scoper, ok := p.checker.(Scoper[T, ID]); ok {
		filters, err := scoper.Scope(ctx, actor)
		if err != nil {
			return nil, err
		}
		if len(filters) > 0 {
			scopeQuery := r3.Query{Filters: filters}
			if len(qarg) > 0 {
				qarg[0] = qarg[0].MergeWith(scopeQuery)
			} else {
				qarg = append(qarg, scopeQuery)
			}
		}
	}

	return agg.Aggregate(ctx, qarg...)
}

// AggregateThroughRelation checks resource-level OpRead and, if the checker is a
// Scoper, injects the owner-scope filters - so relation aggregates fold only
// related rows of owners the actor may see - then delegates to the inner repo.
// Yields r3.ErrRelationAggregateNotSupported when it has no relation aggregation.
func (p *CRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	agg, ok := p.inner.(r3.RelationAggregator)
	if !ok {
		return nil, r3.ErrRelationAggregateNotSupported
	}

	actor := r3.GetActor(ctx)

	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpRead,
		Actor:     actor,
	}); err != nil {
		return nil, err
	}

	// Scoper filters are owner-entity filters, and relation aggregation reads
	// Query.Filters as owner filters - so scope restricts which owners' related
	// rows are counted.
	if scoper, ok := p.checker.(Scoper[T, ID]); ok {
		filters, err := scoper.Scope(ctx, actor)
		if err != nil {
			return nil, err
		}
		if len(filters) > 0 {
			scopeQuery := r3.Query{Filters: filters}
			if len(qarg) > 0 {
				qarg[0] = qarg[0].MergeWith(scopeQuery)
			} else {
				qarg = append(qarg, scopeQuery)
			}
		}
	}

	return agg.AggregateThroughRelation(ctx, relation, qarg...)
}

// Update fetches the existing entity (if IDFunc is set) for entity-aware checks,
// checks OpUpdate, then delegates to inner.Update.
func (p *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	actor := r3.GetActor(ctx)
	req := AccessRequest[T, ID]{
		Operation: OpUpdate,
		Actor:     actor,
	}

	if p.opts.IDFunc != nil {
		id := p.opts.IDFunc(entity)
		req.EntityID = &id
		existing, err := p.inner.Get(ctx, id)
		if err != nil {
			var zero T
			return zero, err
		}
		req.Entity = &existing
	}

	if err := p.checker.Check(ctx, req); err != nil {
		var zero T
		return zero, err
	}

	return p.inner.Update(ctx, entity)
}

// Patch fetches the existing entity (if IDFunc is set) for entity-aware checks,
// checks OpUpdate, then delegates to inner.Patch.
func (p *CRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	actor := r3.GetActor(ctx)
	req := AccessRequest[T, ID]{
		Operation: OpUpdate,
		Actor:     actor,
	}

	if p.opts.IDFunc != nil {
		id := p.opts.IDFunc(entity)
		req.EntityID = &id
		existing, err := p.inner.Get(ctx, id)
		if err != nil {
			var zero T
			return zero, err
		}
		req.Entity = &existing
	}

	if err := p.checker.Check(ctx, req); err != nil {
		var zero T
		return zero, err
	}

	return p.inner.Patch(ctx, entity, fields)
}

// Delete fetches the existing entity (if IDFunc is set) for entity-aware checks,
// checks OpDelete, then delegates to inner.Delete.
func (p *CRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	actor := r3.GetActor(ctx)
	req := AccessRequest[T, ID]{
		Operation: OpDelete,
		Actor:     actor,
		EntityID:  &id,
	}

	if p.opts.IDFunc != nil {
		existing, err := p.inner.Get(ctx, id)
		if err != nil {
			return err
		}
		req.Entity = &existing
	}

	if err := p.checker.Check(ctx, req); err != nil {
		return err
	}

	return p.inner.Delete(ctx, id)
}
