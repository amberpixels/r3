package enginemongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/amberpixels/r3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// setOp is the MongoDB update operator for setting field values.
const setOp = "$set"

// inOp is the MongoDB query operator matching any value in a set.
const inOp = "$in"

// BaseCRUD is a full r3.CRUD[T, ID] backed by a *mongo.Collection. ID must match
// the _id field type (e.g. primitive.ObjectID or string).
type BaseCRUD[T any, ID comparable] struct {
	Collection *mongo.Collection

	r3.DefaultsManager

	Meta   StructMeta
	Config r3.Config
	Raw    *BaseRaw[T, ID]
}

var _ r3.CRUD[any, any] = &BaseCRUD[any, any]{}

// NewBaseCRUD builds a BaseCRUD over the given collection. Accepts optional
// [r3.Option] values.
func NewBaseCRUD[T any, ID comparable](coll *mongo.Collection, opts ...r3.Option) *BaseCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	meta := GetStructMeta[T]()
	// This engine has no codec support yet: fail loudly rather than store un-encoded values.
	r3.RequireCodecSupport(r3.SchemaOf[T](r3.WithSchemaNaming(resolved.Config.Naming)), "r3/mongo")
	return &BaseCRUD[T, ID]{
		Collection:      coll,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Meta:            meta,
		Config:          resolved.Config,
		Raw:             NewBaseRaw[T, ID](coll, meta),
	}
}

// NewBaseCRUDFromDB builds a BaseCRUD, deriving the collection name from T.
// Accepts optional [r3.Option] values.
func NewBaseCRUDFromDB[T any, ID comparable](db *mongo.Database, opts ...r3.Option) *BaseCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	meta := GetStructMeta[T]()
	// This engine has no codec support yet: fail loudly rather than store un-encoded values.
	r3.RequireCodecSupport(r3.SchemaOf[T](r3.WithSchemaNaming(resolved.Config.Naming)), "r3/mongo")
	coll := db.Collection(meta.CollectionName)
	return &BaseCRUD[T, ID]{
		Collection:      coll,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Meta:            meta,
		Config:          resolved.Config,
		Raw:             NewBaseRaw[T, ID](coll, meta),
	}
}

// Create inserts a document and returns it with the driver-generated _id.
func (r *BaseCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	doc := r.Meta.ToBSONDoc(entity, false) // exclude _id, let MongoDB generate it

	result, err := r.Collection.InsertOne(ctx, doc)
	if err != nil {
		return entity, fmt.Errorf("mongo insert: %w", err)
	}

	r.Meta.SetIDValue(&entity, result.InsertedID)

	return entity, nil
}

// Get retrieves a document by its _id, normalizing not-found to [r3.ErrNotFound].
func (r *BaseCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	var entity T

	q := r.MergeGetQuery(qarg...)

	// {_id: id} [AND soft-delete check]
	filter := bson.D{{Key: r.Meta.IDField, Value: id}}
	if r.Meta.SoftDeleteField != "" && !q.IncludeTrashed.Some(true) {
		filter = append(filter, bson.E{Key: r.Meta.SoftDeleteField, Value: r.Meta.liveRecordCondition()})
	}

	opts := options.FindOne()

	if len(q.Fields) > 0 {
		projection := r3FieldsToBSONProjection(q.Fields)
		opts.SetProjection(projection)
	}

	err := r.Collection.FindOne(ctx, filter, opts).Decode(&entity)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return entity, r3.ErrNotFound
		}
		return entity, err
	}

	if len(q.Preloads) > 0 && len(r.Meta.Relations) > 0 {
		entities := []T{entity}
		if err := RunPreloads(ctx, r.Collection.Database(), &r.Meta, &entities, q.Preloads); err != nil {
			return entity, fmt.Errorf("preload failed: %w", err)
		}
		entity = entities[0]
	}

	return entity, nil
}

// liveRecordCondition is the BSON predicate matching records that are NOT
// soft-deleted. A pointer field stores null for live records, so {$eq: nil}
// suffices. A non-pointer field (e.g. time.Time) persists its zero value - the
// zero BSON Date, not null - so the predicate must match both, else every live
// record is filtered out of Get/List/Count.
func (m *StructMeta) liveRecordCondition() bson.D {
	if m.SoftDeleteZero == nil {
		return bson.D{{Key: "$eq", Value: nil}}
	}
	return bson.D{{Key: inOp, Value: bson.A{nil, m.SoftDeleteZero}}}
}

// buildFilter combines the prepared user filters with the soft-delete condition.
// Shared by List and Count; List layers cursor keyset filtering on top.
func (r *BaseCRUD[T, ID]) buildFilter(prep PreparedListQuery) bson.D {
	filter := prep.Filter
	if filter == nil {
		filter = bson.D{}
	}

	if r.Meta.SoftDeleteField != "" && !prep.Query.IncludeTrashed.Some(true) {
		sdFilter := bson.D{{Key: r.Meta.SoftDeleteField, Value: r.Meta.liveRecordCondition()}}
		if len(filter) == 0 {
			filter = sdFilter
		} else {
			filter = bson.D{{Key: "$and", Value: bson.A{filter, sdFilter}}}
		}
	}

	return filter
}

// Count returns the number of documents matching the query's filters.
func (r *BaseCRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	prep, err := PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return 0, err
	}

	total, err := r.Collection.CountDocuments(ctx, r.buildFilter(prep))
	if err != nil {
		return 0, fmt.Errorf("mongo count: %w", err)
	}
	return total, nil
}

// List retrieves documents matching the query.
func (r *BaseCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	prep, err := PrepareListQuery(&r.DefaultsManager, qarg...)
	if err != nil {
		return nil, 0, err
	}

	filter := r.buildFilter(prep)

	// Cursor pagination: AND the keyset filter onto the existing filter.
	if prep.IsCursorPaginated && len(prep.CursorFilter) > 0 {
		if len(filter) == 0 {
			filter = prep.CursorFilter
		} else {
			filter = bson.D{{Key: "$and", Value: bson.A{filter, prep.CursorFilter}}}
		}
	}

	// Count first for offset pagination only; cursor pages return no total.
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

	opts := options.Find()

	if len(prep.Sort) > 0 {
		opts.SetSort(prep.Sort)
	}

	if prep.Projection != nil {
		opts.SetProjection(prep.Projection)
	}

	if prep.IsCursorPaginated {
		opts.SetLimit(prep.CursorLimit)
	} else if prep.IsPaginated {
		opts.SetLimit(prep.Limit)
		opts.SetSkip(prep.Offset)
	}

	cursor, err := r.Collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("mongo find: %w", err)
	}
	defer cursor.Close(ctx)

	var entities []T
	if err := cursor.All(ctx, &entities); err != nil {
		return nil, 0, fmt.Errorf("mongo decode: %w", err)
	}

	if prep.IsCursorPaginated {
		entities, totalCount = r3.FinalizeCountCursor(entities)
	} else {
		entities, totalCount = r3.FinalizeCount(entities, totalCount, prep.IsPaginated)
	}

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
	update := bson.D{{Key: setOp, Value: doc}}

	filter := bson.D{{Key: r.Meta.IDField, Value: idVal}}

	res, err := r.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return entity, fmt.Errorf("mongo update: %w", err)
	}
	if res.MatchedCount == 0 {
		return entity, r3.ErrNotFound
	}
	return entity, nil
}

// Patch updates only the given fields on the document identified by entity's _id,
// then re-fetches and returns it.
func (r *BaseCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	fieldNames := r3.FieldsToStrings(fields)

	fieldNames, err := r.Meta.ValidatePatchFields(fieldNames)
	if err != nil {
		return entity, err
	}

	idVal := r.Meta.IDValue(entity)
	filter := bson.D{{Key: r.Meta.IDField, Value: idVal}}

	setDoc := bson.D{}
	vals := r.Meta.FieldValuesForFields(entity, fieldNames)
	for i, name := range fieldNames {
		setDoc = append(setDoc, bson.E{Key: name, Value: vals[i]})
	}

	update := bson.D{{Key: setOp, Value: setDoc}}

	res, err := r.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return entity, fmt.Errorf("mongo patch: %w", err)
	}
	if res.MatchedCount == 0 {
		return entity, r3.ErrNotFound
	}

	var updated T
	err = r.Collection.FindOne(ctx, filter).Decode(&updated)
	if err != nil {
		return entity, fmt.Errorf("mongo patch refetch: %w", err)
	}

	return updated, nil
}

// Delete removes a document by its _id, or soft-deletes it (setting the
// soft-delete field to now) when the model has one.
func (r *BaseCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	filter := bson.D{{Key: r.Meta.IDField, Value: id}}

	if r.Meta.SoftDeleteField != "" {
		update := bson.D{{Key: setOp, Value: bson.D{
			{Key: r.Meta.SoftDeleteField, Value: time.Now()},
		}}}
		res, err := r.Collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return fmt.Errorf("mongo soft-delete: %w", err)
		}
		if res.MatchedCount == 0 {
			return r3.ErrNotFound
		}
		return nil
	}

	res, err := r.Collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("mongo delete: %w", err)
	}
	if res.DeletedCount == 0 {
		return r3.ErrNotFound
	}
	return nil
}

// Restore un-deletes a soft-deleted document by unsetting its soft-delete field;
// errors if the model has none.
func (r *BaseCRUD[T, ID]) Restore(ctx context.Context, id ID) error {
	if r.Meta.SoftDeleteField == "" {
		return errors.New("r3/engine/mongo: model has no soft-delete field")
	}
	filter := bson.D{{Key: r.Meta.IDField, Value: id}}
	update := bson.D{{Key: "$unset", Value: bson.D{
		{Key: r.Meta.SoftDeleteField, Value: ""},
	}}}
	res, err := r.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("mongo restore: %w", err)
	}
	if res.MatchedCount == 0 {
		return r3.ErrNotFound
	}
	return nil
}

// HardDelete permanently removes a document, bypassing soft-delete.
func (r *BaseCRUD[T, ID]) HardDelete(ctx context.Context, id ID) error {
	filter := bson.D{{Key: r.Meta.IDField, Value: id}}
	res, err := r.Collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("mongo hard-delete: %w", err)
	}
	if res.DeletedCount == 0 {
		return r3.ErrNotFound
	}
	return nil
}

// r3FieldsToBSONProjection converts r3.Fields to a BSON projection, always
// including _id.
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
