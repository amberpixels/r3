package enginemongo

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
)

var _ r3.Aggregator = (*BaseCRUD[any, any])(nil)

// Aggregate computes grouped aggregates via the pipeline: $match (filters +
// soft-delete) → $group → $project (flatten group keys) → $match (having) →
// $sort → $skip/$limit. See [r3.Aggregator] for the query semantics.
//
// Degradations vs SQL: dotted (nested-path) group fields are unsupported, and
// COUNT(DISTINCT field) uses $addToSet, materializing the value set per group -
// fine for the moderate cardinalities aggregates target.
func (r *BaseCRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	prep, err := PrepareListQuery(&r.DefaultsManager, r.CodecSchema, qarg...)
	if err != nil {
		return nil, err
	}
	q := prep.Query
	// No r3.Schema here, but structural validation still applies.
	if err := (r3.Schema{}).ValidateAggregateQuery(q); err != nil {
		return nil, err
	}

	groupNames, err := aggregateGroupNames(q)
	if err != nil {
		return nil, err
	}

	var prefix bson.A
	if filter := r.buildFilter(prep); len(filter) > 0 {
		prefix = append(prefix, bson.D{{Key: matchOp, Value: filter}})
	}

	rows, err := runGroupPipeline(ctx, r.Collection, prefix, groupNames, q)
	if err != nil {
		return nil, err
	}

	// Decode codec'd group-by columns and MIN/MAX aggregates back to their domain
	// values (e.g. a stored unix int -> time.Time).
	if err := r3.DecodeAggregateCodecs(r.CodecSchema, q, rows); err != nil {
		return nil, fmt.Errorf("mongo aggregate: decode codecs: %w", err)
	}
	return rows, nil
}

// aggregateGroupNames returns the query's group-by field names, rejecting nested
// (dotted) names: $project cannot emit a literal dotted key (it means a nested
// path), so flattened group keys must be plain names.
func aggregateGroupNames(q r3.Query) ([]string, error) {
	groupNames := r3.FieldsToStrings(q.GroupBy)
	for _, name := range groupNames {
		if strings.Contains(name, ".") {
			return nil, fmt.Errorf("mongo aggregate: nested group field %q is not supported", name)
		}
	}
	return groupNames, nil
}

// runGroupPipeline appends the $group -> $project -> $match(having) -> $sort ->
// $skip/$limit stages onto prefix, runs the aggregation against coll, and returns
// the normalized rows. prefix carries the stages that select and shape the input
// rows ($match, $lookup, ...). Shared by single-table Aggregate and
// AggregateThroughRelation; it applies no value codecs (the caller decides).
func runGroupPipeline(
	ctx context.Context, coll *mongo.Collection, prefix bson.A, groupNames []string, q r3.Query,
) ([]r3.AggregateRow, error) {
	group, project, err := buildGroupAndProject(groupNames, q.Buckets, q.Aggregates)
	if err != nil {
		return nil, err
	}
	pipeline := append(prefix, //nolint:gocritic // prefix is a fresh per-call slice
		bson.D{{Key: "$group", Value: group}},
		bson.D{{Key: "$project", Value: project}},
	)

	if len(q.Having) > 0 {
		having, err := r3bson.FiltersToBSON(q.Having)
		if err != nil {
			return nil, fmt.Errorf("mongo aggregate: having: %w", err)
		}
		pipeline = append(pipeline, bson.D{{Key: matchOp, Value: having}})
	}

	if sorts := q.AggregateSorts(); len(sorts) > 0 {
		sortDoc, err := r3bson.SortsToBSON(sorts)
		if err != nil {
			return nil, fmt.Errorf("mongo aggregate: sorts: %w", err)
		}
		pipeline = append(pipeline, bson.D{{Key: "$sort", Value: sortDoc}})
	}

	if q.Pagination != nil && q.Pagination.IsPaginated() {
		limit, offset := q.Pagination.ToLimitOffset()
		if offset > 0 {
			pipeline = append(pipeline, bson.D{{Key: "$skip", Value: int64(offset)}})
		}
		pipeline = append(pipeline, bson.D{{Key: "$limit", Value: int64(limit)}})
	}

	cursor, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("mongo aggregate: %w", err)
	}
	defer cursor.Close(ctx)

	var raw []bson.M
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("mongo aggregate decode: %w", err)
	}

	rows := make([]r3.AggregateRow, len(raw))
	for i, doc := range raw {
		row := make(r3.AggregateRow, len(doc))
		for k, v := range doc {
			row[k] = normalizeBSONValue(v)
		}
		rows[i] = row
	}
	return rows, nil
}

// sumOp is the MongoDB accumulator used for COUNT and SUM.
const sumOp = "$sum"

// matchOp is the MongoDB aggregation stage that filters input documents.
const matchOp = "$match"

// buildGroupAndProject translates the aggregate specs into the $group document
// (plain group keys under _id.g<i>, time-bucket keys under _id.b<i>, aggregates
// under their aliases) and the $project that flattens group/bucket keys back to
// field/alias names and turns COUNT(DISTINCT ...) sets into sizes.
func buildGroupAndProject(groupNames []string, buckets r3.Buckets, aggs r3.Aggregates) (bson.D, bson.D, error) {
	var groupID any
	if len(groupNames) > 0 || len(buckets) > 0 {
		id := bson.D{}
		for i, name := range groupNames {
			id = append(id, bson.E{Key: fmt.Sprintf("g%d", i), Value: "$" + name})
		}
		for i, b := range buckets {
			id = append(id, bson.E{Key: fmt.Sprintf("b%d", i), Value: bucketDateTrunc(b)})
		}
		groupID = id
	}

	group := bson.D{{Key: bsonIDField, Value: groupID}}
	project := bson.D{{Key: bsonIDField, Value: 0}}
	for i, name := range groupNames {
		project = append(project, bson.E{Key: name, Value: fmt.Sprintf("$_id.g%d", i)})
	}
	for i, b := range buckets {
		project = append(project, bson.E{Key: b.Alias, Value: fmt.Sprintf("$_id.b%d", i)})
	}

	for _, a := range aggs {
		field := "$" + a.Field.String()
		switch a.Func {
		case r3.AggregateCount:
			if a.Field == nil || a.Field.String() == "" {
				group = append(group, bson.E{Key: a.Alias, Value: bson.D{{Key: sumOp, Value: 1}}})
			} else {
				// COUNT(field): count documents whose value is non-null.
				cond := bson.D{{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$ne", Value: bson.A{field, nil}}}, 1, 0,
				}}}
				group = append(group, bson.E{Key: a.Alias, Value: bson.D{{Key: sumOp, Value: cond}}})
			}
			project = append(project, bson.E{Key: a.Alias, Value: 1})
		case r3.AggregateCountDistinct:
			group = append(group, bson.E{Key: a.Alias, Value: bson.D{{Key: "$addToSet", Value: field}}})
			// Nulls are excluded to match SQL COUNT(DISTINCT col).
			project = append(project, bson.E{Key: a.Alias, Value: bson.D{{Key: "$size", Value: bson.D{
				{Key: "$filter", Value: bson.D{
					{Key: "input", Value: "$" + a.Alias},
					{Key: "cond", Value: bson.D{{Key: "$ne", Value: bson.A{"$$this", nil}}}},
				}},
			}}}})
		case r3.AggregateSum:
			group = append(group, bson.E{Key: a.Alias, Value: bson.D{{Key: sumOp, Value: field}}})
			project = append(project, bson.E{Key: a.Alias, Value: 1})
		case r3.AggregateAvg:
			group = append(group, bson.E{Key: a.Alias, Value: bson.D{{Key: "$avg", Value: field}}})
			project = append(project, bson.E{Key: a.Alias, Value: 1})
		case r3.AggregateMin:
			group = append(group, bson.E{Key: a.Alias, Value: bson.D{{Key: "$min", Value: field}}})
			project = append(project, bson.E{Key: a.Alias, Value: 1})
		case r3.AggregateMax:
			group = append(group, bson.E{Key: a.Alias, Value: bson.D{{Key: "$max", Value: field}}})
			project = append(project, bson.E{Key: a.Alias, Value: 1})
		default:
			return nil, nil, fmt.Errorf("mongo aggregate: unsupported function %v", a.Func)
		}
	}
	return group, project, nil
}

// $dateTrunc keys, shared with the white-box lowering goldens.
const (
	dateTruncOp      = "$dateTrunc"
	dateTruncDateKey = "date"
	dateTruncUnitKey = "unit"
)

// bucketDateTrunc renders a time-bucket group key as a $dateTrunc expression on
// the stored date, with no timezone argument (BSON dates are UTC, i.e. exactly as
// stored - matching the r3 wall-clock contract). Week buckets pin startOfWeek to
// Monday (ISO-8601), the same start the file and SQL engines use.
func bucketDateTrunc(b *r3.BucketSpec) bson.D {
	trunc := bson.D{
		{Key: dateTruncDateKey, Value: "$" + b.Field.String()},
		{Key: dateTruncUnitKey, Value: b.Unit.String()},
	}
	if b.Unit == r3.BucketWeek {
		trunc = append(trunc, bson.E{Key: "startOfWeek", Value: "monday"})
	}
	return bson.D{{Key: dateTruncOp, Value: trunc}}
}

// normalizeBSONValue converts BSON-native scalars to the Go types the
// [r3.AggregateRow] accessors expect (bson.DateTime → UTC time.Time, matching how
// entities decode time fields).
func normalizeBSONValue(v any) any {
	if dt, ok := v.(bson.DateTime); ok {
		return dt.Time().UTC()
	}
	return v
}
