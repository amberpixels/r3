package permissions

import (
	"context"

	"github.com/amberpixels/r3"
)

// crudOps is the full CRUD operation set, in canonical order. AllowedOps probes
// it when no explicit ops are given.
var crudOps = []Operation{OpCreate, OpRead, OpUpdate, OpDelete}

// Allow reports whether op would be permitted for the context actor on the
// given (already-loaded) entity, WITHOUT performing it. It asks the same
// Checker the decorator asks, building the same AccessRequest the decorator's
// corresponding method builds - so an advertised verdict matches enforcement.
//
// It is a projection primitive: use it to publish per-row capabilities (e.g. a
// DTO "can" block a frontend reads to show/hide controls). The backend still
// enforces on every real operation; this only advertises the verdict. The
// verdict is computed against the row the caller holds - if the row has changed
// in storage since it was fetched, advertisement and enforcement can differ at
// action time (inherent to any capability projection).
//
// Scope: for OpRead, Allow additionally consults the Scoper (if the checker is
// one), matching the decorator's Get - an out-of-scope row is not readable.
// Plain column scope filters are matched in memory; a relation ("has") scope
// filter cannot be, so it fails closed (not readable), exactly like the
// decorator's Get without WithIDFunc. Advertising is deliberately pure (no
// I/O): a caller that needs relation scope in the read verdict should scope the
// List that produced the rows, making read-advertise trivially true. For write
// ops only Check runs - the plain Update/Patch/Delete paths never consult the
// read Scoper. (Upsert/PatchWhere do, but they are compositions: probe them as
// OpCreate+OpUpdate and OpUpdate respectively.)
//
// AccessRequest.EntityID is always nil here: a bare Checker has no ID
// extractor. The decorator's Delete (always) and Update/Patch (with WithIDFunc)
// set EntityID during enforcement, so a Checker that keys decisions on
// req.EntityID must be probed via the decorator's [CRUD.Allow], which populates
// it from IDFunc. Checkers deciding from req.Entity fields (the common
// ownership/tenant shape) are advertised faithfully by this helper.
//
// Row-level write advertisement presumes the decorator is configured with
// WithIDFunc (the row-level setup): only then does enforcement fetch the
// existing row and hand it to the Checker as req.Entity. Without IDFunc the
// decorator checks writes at resource level (Entity nil), so an entity-reading
// Checker sees different requests from enforcement than from Allow.
//
// For row-less questions ("may this actor create/list at all?") use
// [AllowResource] - probing Allow with a zero-value entity would let an
// entity-reading policy judge garbage field values.
func Allow[T any, ID comparable](ctx context.Context, checker Checker[T, ID], op Operation, entity T) bool {
	return allowEntity(ctx, checker, op, entity, nil)
}

// AllowResource reports whether the context actor passes the resource-level
// check for op: Entity and EntityID both nil, the exact AccessRequest the
// decorator's List/Count issue for OpRead. Use it for row-less questions
// (nav gating, "show the New button?"). Note it advertises only the
// resource-level verdict - a Scoper may still narrow which rows the actor
// actually sees, and row-level Check rules do not apply.
func AllowResource[T any, ID comparable](ctx context.Context, checker Checker[T, ID], op Operation) bool {
	return checker.Check(ctx, AccessRequest[T, ID]{
		Operation: op,
		Actor:     r3.GetActor(ctx),
	}) == nil
}

// AllowedOps returns the subset of ops permitted for the context actor on
// entity, preserving the given order. Convenience over Allow for building a
// capability set to serialise (a DTO "can" block). With no ops given, it probes
// the full CRUD set (OpCreate, OpRead, OpUpdate, OpDelete); note OpCreate on an
// already-loaded row means "would creating an identical entity be allowed".
func AllowedOps[T any, ID comparable](
	ctx context.Context, checker Checker[T, ID], entity T, ops ...Operation,
) []Operation {
	return allowedOps(ops, func(op Operation) bool {
		return Allow(ctx, checker, op, entity)
	})
}

// Allow reports whether op would be permitted on entity, using this decorator's
// checker - see the package-level [Allow] for semantics. Unlike the bare
// helper, it uses IDFunc (when set) to populate AccessRequest.EntityID for
// non-create ops, matching the request shape the decorator's own methods build,
// so Checkers that read req.EntityID are advertised faithfully too.
func (p *CRUD[T, ID]) Allow(ctx context.Context, op Operation, entity T) bool {
	var id *ID
	if p.opts.IDFunc != nil && op != OpCreate {
		v := p.opts.IDFunc(entity)
		id = &v
	}
	return allowEntity(ctx, p.checker, op, entity, id)
}

// AllowedOps returns the subset of ops permitted on entity, preserving the
// given order - see the package-level [AllowedOps]. With no ops given, it
// probes the full CRUD set.
func (p *CRUD[T, ID]) AllowedOps(ctx context.Context, entity T, ops ...Operation) []Operation {
	return allowedOps(ops, func(op Operation) bool {
		return p.Allow(ctx, op, entity)
	})
}

// allowEntity is the single decision function behind Allow and CRUD.Allow:
// Check with the decorator's request conventions, plus the Scoper fold for
// OpRead. Enforcement and advertisement must route through identical logic -
// the parity test in advertise_test.go pins this.
func allowEntity[T any, ID comparable](
	ctx context.Context, checker Checker[T, ID], op Operation, entity T, id *ID,
) bool {
	actor := r3.GetActor(ctx)
	if err := checker.Check(ctx, AccessRequest[T, ID]{
		Operation: op,
		Actor:     actor,
		Entity:    &entity,
		EntityID:  id,
	}); err != nil {
		return false
	}

	if op != OpRead {
		return true
	}

	// Read verdict = Check AND in-scope, mirroring the decorator's Get: an
	// out-of-scope row is invisible, hence not readable.
	scoper, ok := checker.(Scoper[T, ID])
	if !ok {
		return true
	}
	filters, err := scoper.Scope(ctx, actor)
	if err != nil {
		return false // fail closed, as the decorator's Get fails the read
	}
	if len(filters) == 0 {
		return true
	}
	if containsRelationFilter(filters) {
		// A "has" filter needs the database; pure advertisement does no I/O, so
		// fail closed - exactly like the decorator's Get without WithIDFunc.
		return false
	}
	inScope, matchErr := entityMatchesFilters(&entity, filters)
	return matchErr == nil && inScope
}

// allowedOps filters ops (defaulting to the full CRUD set) through the given
// per-op verdict.
func allowedOps(ops []Operation, allowed func(Operation) bool) []Operation {
	if len(ops) == 0 {
		ops = crudOps
	}
	out := make([]Operation, 0, len(ops))
	for _, op := range ops {
		if allowed(op) {
			out = append(out, op)
		}
	}
	return out
}
