package permissions

import (
	"context"

	"github.com/amberpixels/r3"
)

var (
	_ r3.Upserter[any, any]    = &CRUD[any, any]{}
	_ r3.BulkPatcher[any, any] = &CRUD[any, any]{}
)

// Upsert gates an insert-or-update. Because it can do either, the actor must
// pass BOTH the OpCreate and the OpUpdate check (the stricter of the two wins).
// When the checker is also a Scoper, the entity being written must fall within
// the actor's scope, so a scoped actor cannot upsert onto a row outside its
// scope. Returns r3.ErrUpsertNotSupported when the inner repo has no upsert
// capability.
func (p *CRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	up, ok := p.inner.(r3.Upserter[T, ID])
	if !ok {
		var zero T
		return zero, r3.ErrUpsertNotSupported
	}

	actor := r3.GetActor(ctx)
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpCreate,
		Actor:     actor,
		Entity:    &entity,
	}); err != nil {
		var zero T
		return zero, err
	}
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpUpdate,
		Actor:     actor,
		Entity:    &entity,
	}); err != nil {
		var zero T
		return zero, err
	}

	// Row-level scope: the written entity must satisfy the actor's scope filters.
	// A relationship ("has") scope filter can't be evaluated in memory, so it
	// fails closed (denied) — a scoped write is never allowed to escape scope.
	if scoper, ok := p.checker.(Scoper[T, ID]); ok {
		filters, err := scoper.Scope(ctx, actor)
		if err != nil {
			var zero T
			return zero, err
		}
		if len(filters) > 0 {
			inScope, matchErr := entityMatchesFilters(&entity, filters)
			if matchErr != nil || !inScope {
				var zero T
				return zero, NewAccessDeniedError(OpUpdate, actor, "entity falls outside the actor's scope")
			}
		}
	}

	return up.Upsert(ctx, entity, opts...)
}

// PatchWhere gates a bulk conditional update. The actor must pass the OpUpdate
// check, and — security-critical — when the checker is a Scoper the scope
// filters are AND-ed into the caller's filters so a scoped actor can only mutate
// rows inside its scope. Top-level filters combine with AND, so the effective
// selection is (caller filters AND scope filters). Returns
// r3.ErrBulkPatchNotSupported when the inner repo has no bulk-patch capability.
func (p *CRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	bp, ok := p.inner.(r3.BulkPatcher[T, ID])
	if !ok {
		return 0, r3.ErrBulkPatchNotSupported
	}

	actor := r3.GetActor(ctx)
	if err := p.checker.Check(ctx, AccessRequest[T, ID]{
		Operation: OpUpdate,
		Actor:     actor,
	}); err != nil {
		return 0, err
	}

	if scoper, ok := p.checker.(Scoper[T, ID]); ok {
		scopeFilters, err := scoper.Scope(ctx, actor)
		if err != nil {
			return 0, err
		}
		if len(scopeFilters) > 0 {
			// Never mutate the caller's slice: build the combined filter fresh.
			combined := make(r3.Filters, 0, len(filters)+len(scopeFilters))
			combined = append(combined, filters...)
			combined = append(combined, scopeFilters...)
			filters = combined
		}
	}

	return bp.PatchWhere(ctx, filters, entity, fields)
}
