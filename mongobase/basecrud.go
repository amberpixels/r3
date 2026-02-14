package mongobase

import (
	"context"
	"fmt"
	"time"

	"github.com/amberpixels/r3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// BaseCRUD is a generic CRUD repository backed by MongoDB.
// It implements the full r3.CRUD[T, ID] interface using the MongoDB Go driver v2.
//
// The struct type T must use `bson` struct tags for field mapping (or `r3` tags).
// The ID type should match the MongoDB _id field type (e.g. primitive.ObjectID or string).
type BaseCRUD[T any, ID comparable] struct {
	Collection *mongo.Collection

	r3.DefaultsManager

	Meta StructMeta
	Raw  *BaseRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BaseCRUD[any, any]{}

// NewBaseCRUD creates a new BaseCRUD with the given MongoDB collection.
// If collection is nil, it derives the collection name from the struct type T
// and uses the provided database.
func NewBaseCRUD[T any, ID comparable](coll *mongo.Collection) *BaseCRUD[T, ID] {
	meta := GetStructMeta[T]()
	return &BaseCRUD[T, ID]{
		Collection:      coll,
		DefaultsManager: r3.NewDefaultsManager(),
		Meta:            meta,
		Raw:             NewBaseRaw[T, ID](coll, meta),
	}
}

// NewBaseCRUDFromDB creates a new BaseCRUD using a database, deriving the collection
// name automatically from the struct type T.
func NewBaseCRUDFromDB[T any, ID comparable](db *mongo.Database) *BaseCRUD[T, ID] {
	meta := GetStructMeta[T]()
	coll := db.Collection(meta.CollectionName)
	return &BaseCRUD[T, ID]{
		Collection:      coll,
		DefaultsManager: r3.NewDefaultsManager(),
		Meta:            meta,
		Raw:             NewBaseRaw[T, ID](coll, meta),
	}
}

// Create inserts a new document into MongoDB and returns it with the generated _id.
func (r *BaseCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	doc := r.Meta.ToBSONDoc(entity, false) // exclude _id, let MongoDB generate it

	result, err := r.Collection.InsertOne(ctx, doc)
	if err != nil {
		return entity, fmt.Errorf("mongo insert: %w", err)
	}

	// Set the _id on the entity
	r.Meta.SetIDValue(&entity, result.InsertedID)

	return entity, nil
}

// Get retrieves a document by its _id.
func (r *BaseCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	// Build filter: {_id: id} [AND soft-delete check]
	filter := bson.D{{Key: r.Meta.IDField, Value: id}}
	if r.Meta.SoftDeleteField != "" && !q.IncludeTrashed.Some(true) {
		filter = append(filter, bson.E{Key: r.Meta.SoftDeleteField, Value: bson.D{{Key: "$eq", Value: nil}}})
	}

	// Build find options
	opts := options.FindOne()

	// Field projection
	if len(q.Fields) > 0 {
		projection := r3FieldsToBSONProjection(q.Fields)
		opts.SetProjection(projection)
	}

	err := r.Collection.FindOne(ctx, filter, opts).Decode(&entity)
	if err != nil {
		return entity, err
	}

	// Run preloads
	if len(q.Preloads) > 0 && len(r.Meta.Relations) > 0 {
		entities := []T{entity}
		if err := RunPreloads(ctx, r.Collection.Database(), &r.Meta, &entities, q.Preloads); err != nil {
			return entity, fmt.Errorf("preload failed: %w", err)
		}
		entity = entities[0]
	}

	return entity, nil
}

// List retrieves documents based on the provided query parameters.
func (r *BaseCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	// Build the filter
	filter := prep.Filter
	if filter == nil {
		filter = bson.D{}
	}

	// Add soft-delete filter
	if r.Meta.SoftDeleteField != "" && !prep.Query.IncludeTrashed.Some(true) {
		sdFilter := bson.D{{Key: r.Meta.SoftDeleteField, Value: bson.D{{Key: "$eq", Value: nil}}}}
		if len(filter) == 0 {
			filter = sdFilter
		} else {
			filter = bson.D{{Key: "$and", Value: bson.A{filter, sdFilter}}}
		}
	}

	// Count first if paginated
	var totalCount int64
	if prep.IsPaginated {
		totalCount, err = r.Collection.CountDocuments(ctx, filter)
		if err != nil {
			return nil, 0, fmt.Errorf("mongo count: %w", err)
		}
		if totalCount == 0 {
			return nil, 0, nil
		}
	}

	// Build find options
	opts := options.Find()

	if len(prep.Sort) > 0 {
		opts.SetSort(prep.Sort)
	}

	if prep.Projection != nil {
		opts.SetProjection(prep.Projection)
	}

	if prep.IsPaginated {
		opts.SetLimit(prep.Limit)
		opts.SetSkip(prep.Offset)
	}

	// Execute the query
	cursor, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("mongo find: %w", err)
	}
	defer cursor.Close(ctx)

	var entities []T
	if err := cursor.All(ctx, &entities); err != nil {
		return nil, 0, fmt.Errorf("mongo decode: %w", err)
	}

	entities, totalCount = r3.FinalizeCount(entities, totalCount, prep.IsPaginated)

	// Run preloads
	if len(prep.Query.Preloads) > 0 && len(r.Meta.Relations) > 0 {
		if err := RunPreloads(ctx, r.Collection.Database(), &r.Meta, &entities, prep.Query.Preloads); err != nil {
			return nil, 0, fmt.Errorf("preload failed: %w", err)
		}
	}

	return entities, totalCount, nil
}

// Update replaces an existing document identified by its _id.
func (r *BaseCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	idVal := r.Meta.IDValue(entity)

	doc := r.Meta.ToBSONDoc(entity, false)
	update := bson.D{{Key: "$set", Value: doc}}

	filter := bson.D{{Key: r.Meta.IDField, Value: idVal}}

	_, err := r.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return entity, fmt.Errorf("mongo update: %w", err)
	}
	return entity, nil
}

// Patch performs a partial update, modifying only the fields specified by fields.
// The entity must have its _id set. Returns the entity after the update.
func (r *BaseCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	fieldNames := r3.FieldsToStrings(fields)

	fieldNames, err := r.Meta.ValidatePatchFields(fieldNames)
	if err != nil {
		return entity, err
	}

	idVal := r.Meta.IDValue(entity)
	filter := bson.D{{Key: r.Meta.IDField, Value: idVal}}

	// Build $set document with only the specified fields
	setDoc := bson.D{}
	vals := r.Meta.FieldValuesForFields(entity, fieldNames)
	for i, name := range fieldNames {
		setDoc = append(setDoc, bson.E{Key: name, Value: vals[i]})
	}

	update := bson.D{{Key: "$set", Value: setDoc}}

	_, err = r.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return entity, fmt.Errorf("mongo patch: %w", err)
	}

	// Re-fetch the updated entity
	var updated T
	err = r.Collection.FindOne(ctx, filter).Decode(&updated)
	if err != nil {
		return entity, fmt.Errorf("mongo patch refetch: %w", err)
	}

	return updated, nil
}

// Delete removes a document by its _id.
// If the model has a soft-delete field, it sets the field to the current time
// instead of actually deleting the document.
func (r *BaseCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	filter := bson.D{{Key: r.Meta.IDField, Value: id}}

	if r.Meta.SoftDeleteField != "" {
		// Soft-delete: set deleted_at to now
		update := bson.D{{Key: "$set", Value: bson.D{
			{Key: r.Meta.SoftDeleteField, Value: time.Now()},
		}}}
		_, err := r.Collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return fmt.Errorf("mongo soft-delete: %w", err)
		}
		return nil
	}

	// Hard delete
	_, err := r.Collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("mongo delete: %w", err)
	}
	return nil
}

// r3FieldsToBSONProjection converts r3.Fields to a BSON projection document.
func r3FieldsToBSONProjection(fields r3.Fields) bson.D {
	if len(fields) == 0 {
		return nil
	}

	projection := make(bson.D, 0, len(fields)+1)
	hasID := false
	for _, f := range fields {
		if f == nil {
			continue
		}
		name := f.String()
		if name == "_id" {
			hasID = true
		}
		projection = append(projection, bson.E{Key: name, Value: 1})
	}
	if !hasID {
		projection = append(bson.D{{Key: "_id", Value: 1}}, projection...)
	}
	return projection
}
