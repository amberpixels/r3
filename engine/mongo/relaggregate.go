package enginemongo

import (
	"context"
	"fmt"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var _ r3.RelationAggregator = (*BaseCRUD[any, any])(nil)

// AggregateThroughRelation aggregates the entity's related rows through a declared
// relation, grouped and folded over the related base collection (the child
// collection for has-many, the join collection for many-to-many). It mirrors
// drivers/gorm/relaggregate.go; see [r3.RelationAggregator] for the query
// semantics.
//
// Mongo has no server-side join, so owner restriction and target soft-delete
// exclusion are done as pre-query key sets (like the relationship filters), then
// the shared $group pipeline runs over the base collection.
func (r *BaseCRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	rel, ok := findRelation(&r.Meta, relation)
	if !ok {
		return nil, fmt.Errorf("relation aggregate: unknown relation %q on %s", relation, r.Meta.CollectionName)
	}

	baseColl, err := r.relationAggregateBase(rel)
	if err != nil {
		return nil, err
	}

	// Merge only the caller queries, not the entity's default list query: its
	// default sorts and pagination name the owner's own fields, absent from the
	// related base collection.
	var q r3.Query
	for _, a := range qarg {
		q = q.MergeWith(a)
	}
	// Structural (shape-only) validation only: the related base collection carries
	// no r3 schema (a bare join collection or an unimported target).
	if err := (r3.Schema{}).ValidateAggregateQuery(q); err != nil {
		return nil, err
	}
	groupNames, err := aggregateGroupNames(q)
	if err != nil {
		return nil, err
	}

	prefix, err := r.relationAggregatePrefix(ctx, rel, q)
	if err != nil {
		return nil, err
	}

	// The related base collection has no r3 codec schema, so no codec decode.
	return runGroupPipeline(ctx, r.Collection.Database().Collection(baseColl), prefix, groupNames, q)
}

// relationAggregateBase returns the collection whose rows are grouped: the child
// collection for has-many, the join collection for many-to-many. belongs-to is not
// aggregatable (each owner has at most one related row).
func (r *BaseCRUD[T, ID]) relationAggregateBase(rel RelationMeta) (string, error) {
	switch rel.Kind {
	case RelHasMany:
		return rel.TargetMeta.CollectionName, nil
	case RelManyToMany:
		if rel.JoinTable == "" || rel.RefField == "" {
			return "", fmt.Errorf(
				"relation aggregate: many-to-many relation %q is missing join table or ref field", rel.FieldName)
		}
		return rel.JoinTable, nil
	default:
		return "", fmt.Errorf(
			"relation aggregate: relation %q is not aggregatable "+
				"(only has-many and many-to-many are supported)", rel.FieldName)
	}
}

// relationAggregatePrefix builds the pipeline stages that select which related
// rows to fold: an owner-key restriction ($match on the base's owner FK against the
// owners matching q.Filters) and a soft-delete exclusion of the related rows.
func (r *BaseCRUD[T, ID]) relationAggregatePrefix(
	ctx context.Context, rel RelationMeta, q r3.Query,
) (bson.A, error) {
	var prefix bson.A

	// Owner filters restrict which owners' related rows are aggregated - this is
	// where a permissions Scoper's owner filters land. Empty filters -> all owners.
	if len(q.Filters) > 0 {
		ownerKeys, err := r.ownerKeys(ctx, q.Filters)
		if err != nil {
			return nil, err
		}
		prefix = append(prefix, bson.D{{Key: matchOp, Value: bson.D{
			{Key: rel.FKField, Value: bson.D{{Key: inOp, Value: ownerKeys}}},
		}}})
	}

	// Exclude soft-deleted related rows when the relation declares a soft-delete
	// field on the target.
	if sd := rel.TargetMeta.SoftDeleteField; sd != "" {
		match, err := r.relationSoftDeleteMatch(ctx, rel, sd)
		if err != nil {
			return nil, err
		}
		if len(match) > 0 {
			prefix = append(prefix, bson.D{{Key: matchOp, Value: match}})
		}
	}

	return prefix, nil
}

// ownerKeys returns the primary keys of the owners matching ownerFilters (with the
// owner's own soft-delete guard applied and any relationship filters lowered).
func (r *BaseCRUD[T, ID]) ownerKeys(ctx context.Context, ownerFilters r3.Filters) ([]any, error) {
	if hasRelationFilter(ownerFilters) {
		lowered, err := r.lowerRelationFilters(ctx, ownerFilters)
		if err != nil {
			return nil, err
		}
		ownerFilters = lowered
	}
	// Owner filters are on the owner entity, so encode its codec'd args to stored
	// form (e.g. a time.Time bound against a unixtime int column).
	ownerFilters, err := r3.EncodeFilterCodecs(r.CodecSchema, ownerFilters)
	if err != nil {
		return nil, fmt.Errorf("relation aggregate: encode owner filter codecs: %w", err)
	}
	filter, err := r3bson.FiltersToBSON(ownerFilters)
	if err != nil {
		return nil, fmt.Errorf("relation aggregate: translate owner filters: %w", err)
	}
	filter = r.applyLiveFilter(filter, false)

	return distinctKeys(ctx, r.Collection, r.Meta.IDField, filter)
}

// relationSoftDeleteMatch builds the $match that keeps only related rows whose
// target is live. For has-many the base IS the target, so it is a direct predicate;
// for many-to-many the soft-delete lives on the target (not the join), so it
// resolves to the live target keys and matches the join's ref field against them.
func (r *BaseCRUD[T, ID]) relationSoftDeleteMatch(ctx context.Context, rel RelationMeta, sd string) (bson.D, error) {
	if rel.Kind == RelHasMany {
		return bson.D{{Key: sd, Value: rel.TargetMeta.liveRecordCondition()}}, nil
	}

	// many-to-many: fetch the live target keys, keep join rows referencing them.
	targetColl := r.Collection.Database().Collection(rel.TargetMeta.CollectionName)
	liveFilter := bson.D{{Key: sd, Value: rel.TargetMeta.liveRecordCondition()}}
	liveKeys, err := distinctKeys(ctx, targetColl, rel.TargetMeta.IDField, liveFilter)
	if err != nil {
		return nil, err
	}
	return bson.D{{Key: rel.RefField, Value: bson.D{{Key: inOp, Value: liveKeys}}}}, nil
}

// distinctKeys returns the distinct, non-null values of field in coll among the
// documents matching filter.
func distinctKeys(ctx context.Context, coll *mongo.Collection, field string, filter bson.D) ([]any, error) {
	var values []any
	if err := coll.Distinct(ctx, field, filter).Decode(&values); err != nil {
		return nil, fmt.Errorf("resolve relation keys: %w", err)
	}
	keys := make([]any, 0, len(values))
	for _, v := range values {
		if v == nil {
			continue
		}
		keys = append(keys, v)
	}
	return keys, nil
}
