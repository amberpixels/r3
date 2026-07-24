package enginemongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
)

// This file resolves relationship ("has") filters the same backend-agnostic way
// the GORM driver does: instead of a server-side join (which Mongo lacks), a
// relationship filter runs as a small pre-query yielding the matching owner keys,
// then is rewritten to a plain key-set In/NotIn filter that r3bson already
// translates. It mirrors drivers/gorm/relfilter.go.

// prepareListQuery merges defaults with the caller args, lowers any relationship
// ("has") filters into key-set filters via a pre-query, then builds the BSON list
// components. The lowering step is the only difference from the pure
// [PrepareListQuery]; it needs a context and the database, so it lives here.
func (r *BaseCRUD[T, ID]) prepareListQuery(ctx context.Context, qarg ...r3.Query) (PreparedListQuery, error) {
	q := r.MergeListQuery(qarg...)
	if hasRelationFilter(q.Filters) {
		lowered, err := r.lowerRelationFilters(ctx, q.Filters)
		if err != nil {
			return PreparedListQuery{}, err
		}
		q.Filters = lowered
	}
	return PrepareMergedListQuery(r.CodecSchema, q)
}

// lowerRelationFilters returns a copy of filters with every relationship filter
// resolved against the database and replaced by an equivalent In/NotIn filter,
// recursing through And/Or groups. The repository's own Meta (already including any
// declared relations) supplies the relation metadata.
func (r *BaseCRUD[T, ID]) lowerRelationFilters(ctx context.Context, filters r3.Filters) (r3.Filters, error) {
	out := make(r3.Filters, len(filters))
	for i, f := range filters {
		lowered, err := r.lowerRelationFilter(ctx, f)
		if err != nil {
			return nil, err
		}
		out[i] = lowered
	}
	return out, nil
}

func (r *BaseCRUD[T, ID]) lowerRelationFilter(ctx context.Context, f *r3.FilterSpec) (*r3.FilterSpec, error) {
	switch {
	case f == nil:
		return nil, nil //nolint:nilnil // a nil child stays nil; callers tolerate it
	case f.Relation != "":
		return r.resolveRelationFilter(ctx, f)
	case len(f.And) > 0:
		children, err := r.lowerRelationFilters(ctx, f.And)
		if err != nil {
			return nil, err
		}
		return &r3.FilterSpec{And: children}, nil
	case len(f.Or) > 0:
		children, err := r.lowerRelationFilters(ctx, f.Or)
		if err != nil {
			return nil, err
		}
		return &r3.FilterSpec{Or: children}, nil
	default:
		return f, nil
	}
}

// resolveRelationFilter resolves one relationship filter into a key-set filter on
// a field of the owner collection: an In for Has, a NotIn (anti-join) for HasNo.
// "field IN []" matches nothing and "field NOT IN []" matches everything, so the
// empty-key set falls out correctly for both directions.
func (r *BaseCRUD[T, ID]) resolveRelationFilter(ctx context.Context, f *r3.FilterSpec) (*r3.FilterSpec, error) {
	rel, ok := findRelation(&r.Meta, f.Relation)
	if !ok {
		return nil, fmt.Errorf("relationship filter: unknown relation %q on %s", f.Relation, r.Meta.CollectionName)
	}
	db := r.Collection.Database()

	switch rel.Kind {
	case RelManyToMany:
		// Two hops: related rows matching the inner filter -> their PKs -> the join
		// collection -> the owner FKs. WHERE owner._id [NOT] IN (those FKs).
		targetKeys, err := selectRelationKeys(
			ctx, db.Collection(rel.TargetMeta.CollectionName), rel.TargetMeta.IDField, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		ownerKeys := []any{}
		if len(targetKeys) > 0 {
			ownerKeys, err = selectRelationKeys(
				ctx, db.Collection(rel.JoinTable), rel.FKField, r3.Filters{r3.In(rel.RefField, targetKeys)},
			)
			if err != nil {
				return nil, err
			}
		}
		return keySetFilter(r.Meta.IDField, ownerKeys, f.RelationNegate), nil

	case RelBelongsTo:
		// The FK lives on the owner row: WHERE owner.fk [NOT] IN (matching target
		// PKs). For the negated case a null FK means "has no related row", so it must
		// be included - NOT IN alone drops it (null NOT IN (...) is never true).
		targetKeys, err := selectRelationKeys(
			ctx, db.Collection(rel.TargetMeta.CollectionName), rel.TargetMeta.IDField, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		if f.RelationNegate {
			return r3.Or(r3.NotIn(rel.FKField, targetKeys), r3.Eq(rel.FKField, nil)), nil
		}
		return r3.In(rel.FKField, targetKeys), nil

	case RelHasMany:
		// The FK lives on the child row: collect the FK values of matching children,
		// then WHERE owner._id [NOT] IN (those FKs).
		ownerKeys, err := selectRelationKeys(
			ctx, db.Collection(rel.TargetMeta.CollectionName), rel.FKField, f.RelationFilter,
		)
		if err != nil {
			return nil, err
		}
		return keySetFilter(r.Meta.IDField, ownerKeys, f.RelationNegate), nil

	default:
		return nil, fmt.Errorf("relationship filter: unsupported relation kind on %q", f.Relation)
	}
}

// keySetFilter builds the owner-side key-set filter: NotIn for an anti-join
// (HasNo), In otherwise. The owner key field (_id) is never null, so no null guard
// is needed (unlike the belongs-to FK case).
func keySetFilter(field string, keys []any, negate bool) *r3.FilterSpec {
	if negate {
		return r3.NotIn(field, keys)
	}
	return r3.In(field, keys)
}

// selectRelationKeys returns the distinct, non-null values of field in coll among
// documents matching inner. An empty inner filter selects all documents. inner
// must not itself contain relationship filters.
func selectRelationKeys(
	ctx context.Context, coll *mongo.Collection, field string, inner r3.Filters,
) ([]any, error) {
	filter, err := r3bson.FiltersToBSON(inner)
	if err != nil {
		return nil, fmt.Errorf("translate relation filter: %w", err)
	}
	return distinctKeys(ctx, coll, field, filter)
}

// findRelation looks up a relation by name in meta's Relations.
func findRelation(meta *StructMeta, name string) (RelationMeta, bool) {
	for _, rel := range meta.Relations {
		if rel.FieldName == name {
			return rel, true
		}
	}
	return RelationMeta{}, false
}

// hasRelationFilter reports whether any filter (recursing into AND/OR groups) is a
// relationship ("has") filter. Used to keep the relation-free path a no-op and to
// reject relationship filters where they are not supported (e.g. PatchWhere).
func hasRelationFilter(filters r3.Filters) bool {
	for _, f := range filters {
		if f == nil {
			continue
		}
		if f.Relation != "" {
			return true
		}
		if hasRelationFilter(f.And) || hasRelationFilter(f.Or) {
			return true
		}
	}
	return false
}
