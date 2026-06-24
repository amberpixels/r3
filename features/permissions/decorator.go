package permissions

import (
	"context"

	"github.com/amberpixels/r3"
)

// CRUD is a decorator that wraps any r3.CRUD[T, ID] and gates every operation
// through a user-defined Checker interface. The decorator is stateless -- it
// doesn't store roles or permissions. It asks the Checker "is this allowed?"
// before delegating to the inner CRUD.
//
// It transparently satisfies the r3.CRUD[T, ID] interface, so it can be used
// as a drop-in replacement for any CRUD repository.
//
// Usage:
//
//	repo := permissions.WithPermissions[Post, int64](
//	    innerRepo,
//	    myChecker,
//	    permissions.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
//	)
type CRUD[T any, ID comparable] struct {
	inner   r3.CRUD[T, ID]
	checker Checker[T, ID]
	opts    Options[T, ID]
}

// Compile-time check that CRUD satisfies r3.CRUD.
var _ r3.CRUD[any, any] = &CRUD[any, any]{}

// WithPermissions wraps an existing r3.CRUD with permission checking.
// The checker parameter is the authorization policy that gates every operation.
//
// Example:
//
//	repo := permissions.WithPermissions[Post, int64](
//	    innerRepo,
//	    permissions.AllowAll[Post, int64](),
//	)
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

// Inner returns the underlying CRUD repository (unwrapped).
// Useful when you need to bypass permission checking for a specific operation.
func (p *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return p.inner
}

// Unwrap returns the wrapped CRUD so capability detection and transaction
// propagation can walk the decorator chain.
func (p *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return p.inner
}

// Rewrap rebuilds this decorator around a different inner CRUD (used to
// re-apply the permission layer on top of a transaction-bound CRUD).
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

// Get delegates to inner.Get first (need entity for row-level check),
// then checks OpRead permission. If denied, the entity is not leaked.
//
// If the checker also implements Scoper, the fetched entity is verified against
// the scope filters — consistent with List/Count, which apply the same filters
// at the database level. An entity outside the actor's scope is reported as
// [r3.ErrNotFound] (invisible), exactly as List would omit it. The scope filters
// are evaluated in-memory against the fetched entity; an unevaluable filter
// fails closed (treated as out of scope).
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

// entityInScope reports whether the already-fetched entity (id/result) satisfies
// the actor's scope filters. Plain column filters are matched in memory (fast,
// no extra query). A relationship ("has") filter can't be evaluated in memory,
// so scope is verified with a query that applies the same filters List/Count use
// — consistent semantics at the cost of one extra query (bounded by the in-scope
// row count; a future native-EXISTS path can pin by PK instead).
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

// List checks OpRead permission (resource-level), optionally injects scope
// filters if the checker implements Scoper, then delegates to inner.List.
func (p *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	actor := r3.GetActor(ctx)

	// Resource-level check: can this actor list this resource type at all?
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpRead,
		Actor:     actor,
	}); err != nil {
		return nil, 0, err
	}

	// If the checker also implements Scoper, inject scope filters.
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

// Count checks OpRead permission (resource-level), optionally injects scope
// filters if the checker implements Scoper, then delegates to inner.Count.
func (p *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	actor := r3.GetActor(ctx)

	// Resource-level check: can this actor read this resource type at all?
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpRead,
		Actor:     actor,
	}); err != nil {
		return 0, err
	}

	// If the checker also implements Scoper, inject scope filters.
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

// Update fetches the existing entity (if IDFunc is set) for entity-aware
// checks, then checks OpUpdate permission, then delegates to inner.Update.
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

// Patch fetches the existing entity (if IDFunc is set) for entity-aware
// checks, then checks OpUpdate permission, then delegates to inner.Patch.
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

// Delete fetches the existing entity (if IDFunc is set) for entity-aware
// checks, then checks OpDelete permission, then delegates to inner.Delete.
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
