package enginemongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// BaseRaw is the escape hatch for arbitrary operations against the underlying
// collection.
type BaseRaw[T any, ID any] struct {
	Collection *mongo.Collection
	Meta       StructMeta
}

// NewBaseRaw builds a BaseRaw over coll.
func NewBaseRaw[T any, ID comparable](coll *mongo.Collection, meta StructMeta) *BaseRaw[T, ID] {
	return &BaseRaw[T, ID]{
		Collection: coll,
		Meta:       meta,
	}
}

// Find executes a find query with the given filter and returns matching documents.
func (r *BaseRaw[T, ID]) Find(
	ctx context.Context,
	filter any,
	opts ...options.Lister[options.FindOptions],
) ([]T, error) {
	cursor, err := r.Collection.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var entities []T
	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// FindOne executes a find query expected to return a single document.
func (r *BaseRaw[T, ID]) FindOne(
	ctx context.Context,
	filter any,
	opts ...options.Lister[options.FindOneOptions],
) (T, error) {
	var entity T
	err := r.Collection.FindOne(ctx, filter, opts...).Decode(&entity)
	if err != nil {
		return entity, err
	}
	return entity, nil
}

// InsertOne inserts a single document.
func (r *BaseRaw[T, ID]) InsertOne(ctx context.Context, doc any) (*mongo.InsertOneResult, error) {
	return r.Collection.InsertOne(ctx, doc)
}

// UpdateOne updates a single document matching the filter.
func (r *BaseRaw[T, ID]) UpdateOne(ctx context.Context, filter any, update any) (*mongo.UpdateResult, error) {
	return r.Collection.UpdateOne(ctx, filter, update)
}

// DeleteOne deletes a single document matching the filter.
func (r *BaseRaw[T, ID]) DeleteOne(ctx context.Context, filter any) (*mongo.DeleteResult, error) {
	return r.Collection.DeleteOne(ctx, filter)
}

// Aggregate executes an aggregation pipeline and decodes results into T.
func (r *BaseRaw[T, ID]) Aggregate(ctx context.Context, pipeline any) ([]T, error) {
	cursor, err := r.Collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var entities []T
	if err := cursor.All(ctx, &entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// CountDocuments returns the count of documents matching the filter.
func (r *BaseRaw[T, ID]) CountDocuments(ctx context.Context, filter any) (int64, error) {
	return r.Collection.CountDocuments(ctx, filter)
}

// Col returns the underlying *mongo.Collection for fully custom usage.
func (r *BaseRaw[T, ID]) Col() *mongo.Collection { return r.Collection }

// Distinct returns the distinct values for a given field.
func (r *BaseRaw[T, ID]) Distinct(ctx context.Context, fieldName string, filter any) *mongo.DistinctResult {
	return r.Collection.Distinct(ctx, fieldName, filter)
}

// UpdateMany updates all documents matching the filter.
func (r *BaseRaw[T, ID]) UpdateMany(ctx context.Context, filter any, update any) (*mongo.UpdateResult, error) {
	return r.Collection.UpdateMany(ctx, filter, update)
}

// DeleteMany deletes all documents matching the filter.
func (r *BaseRaw[T, ID]) DeleteMany(ctx context.Context, filter any) (*mongo.DeleteResult, error) {
	return r.Collection.DeleteMany(ctx, filter)
}

// BulkWrite executes a bulk write operation.
func (r *BaseRaw[T, ID]) BulkWrite(ctx context.Context, models []mongo.WriteModel) (*mongo.BulkWriteResult, error) {
	return r.Collection.BulkWrite(ctx, models)
}

// FindOneAndUpdate atomically finds a document and updates it, returning the result.
func (r *BaseRaw[T, ID]) FindOneAndUpdate(
	ctx context.Context,
	filter any,
	update any,
	opts ...options.Lister[options.FindOneAndUpdateOptions],
) (T, error) {
	var entity T
	err := r.Collection.FindOneAndUpdate(ctx, filter, update, opts...).Decode(&entity)
	return entity, err
}

// FindOneAndDelete atomically finds a document and deletes it, returning the deleted document.
func (r *BaseRaw[T, ID]) FindOneAndDelete(
	ctx context.Context,
	filter any,
	opts ...options.Lister[options.FindOneAndDeleteOptions],
) (T, error) {
	var entity T
	err := r.Collection.FindOneAndDelete(ctx, filter, opts...).Decode(&entity)
	return entity, err
}

// BSONFilter creates a simple bson.D filter from key-value pairs.
// Usage: BSONFilter("status", "active", "age", bson.D{{"$gt", 18}}).
func BSONFilter(kv ...any) bson.D {
	if len(kv)%2 != 0 {
		panic("BSONFilter requires an even number of arguments (key-value pairs)")
	}
	doc := make(bson.D, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			panic("BSONFilter keys must be strings")
		}
		doc = append(doc, bson.E{Key: key, Value: kv[i+1]})
	}
	return doc
}
