package enginemongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/amberpixels/r3"
)

// setOp is the MongoDB update operator for setting field values.
const setOp = "$set"

// inOp is the MongoDB query operator matching any value in a set.
const inOp = "$in"

// BaseCRUD is a full r3.CRUD[T, ID] backed by a *mongo.Collection. ID must match
// the _id field type (e.g. primitive.ObjectID or string).
type BaseCRUD[T any, ID comparable] struct {
	r3.DefaultsManager

	Collection *mongo.Collection

	Meta   StructMeta
	Config r3.Config
	Raw    *BaseRaw[T, ID]

	// CodecSchema is the minimal schema (codec'd fields, keyed by bson name) that
	// drives value-codec encoding of filter and cursor arguments. Zero when the
	// type declares no codecs.
	CodecSchema r3.Schema
}

var _ r3.CRUD[any, any] = &BaseCRUD[any, any]{}

// NewBaseCRUD builds a BaseCRUD over the given collection. Accepts optional
// [r3.Option] values.
func NewBaseCRUD[T any, ID comparable](coll *mongo.Collection, opts ...r3.Option) *BaseCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	meta := WithDeclaredRelations(GetStructMeta[T](), resolved.Relations)
	return &BaseCRUD[T, ID]{
		Collection:      coll,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Meta:            meta,
		Config:          resolved.Config,
		Raw:             NewBaseRaw[T, ID](coll, meta),
		CodecSchema:     meta.codecSchema(),
	}
}

// NewBaseCRUDFromDB builds a BaseCRUD, deriving the collection name from T.
// Accepts optional [r3.Option] values.
func NewBaseCRUDFromDB[T any, ID comparable](db *mongo.Database, opts ...r3.Option) *BaseCRUD[T, ID] {
	resolved := r3.ResolveOptions(opts...)
	meta := WithDeclaredRelations(GetStructMeta[T](), resolved.Relations)
	coll := db.Collection(meta.CollectionName)
	return &BaseCRUD[T, ID]{
		Collection:      coll,
		DefaultsManager: r3.NewDefaultsManagerWithConfig(resolved.Config),
		Meta:            meta,
		Config:          resolved.Config,
		Raw:             NewBaseRaw[T, ID](coll, meta),
		CodecSchema:     meta.codecSchema(),
	}
}

// Create inserts a document and returns it with its _id. A pre-assigned
// non-zero _id (e.g. an app-generated UUID string PK, as used by
// features/history) is stored as-is; a zero _id is excluded from the insert so
// MongoDB generates one, which is then written back onto the entity.
func (r *BaseCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	hasID := !r.idIsZero(entity)
	doc := r.Meta.ToBSONDoc(entity, hasID)
	if err := r.Meta.encodeWriteDoc(doc); err != nil {
		return entity, err
	}

	result, err := r.Collection.InsertOne(ctx, doc)
	if err != nil {
		return entity, fmt.Errorf("mongo insert: %w", err)
	}

	if !hasID {
		r.Meta.SetIDValue(&entity, result.InsertedID)
	}

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

	if r.Meta.HasCodecs() {
		var raw bson.M
		if err := r.Collection.FindOne(ctx, filter, opts).Decode(&raw); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return entity, r3.ErrNotFound
			}
			return entity, err
		}
		if err := unmarshalWithCodecs(&r.Meta, raw, &entity); err != nil {
			return entity, err
		}
	} else {
		err := r.Collection.FindOne(ctx, filter, opts).Decode(&entity)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return entity, r3.ErrNotFound
			}
			return entity, err
		}
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
	return r.applyLiveFilter(filter, prep.Query.IncludeTrashed.Some(true))
}

// applyLiveFilter ANDs the "not soft-deleted" condition onto filter, unless
// includeTrashed is set or the model has no soft-delete field.
func (r *BaseCRUD[T, ID]) applyLiveFilter(filter bson.D, includeTrashed bool) bson.D {
	if r.Meta.SoftDeleteField == "" || includeTrashed {
		return filter
	}
	sdFilter := bson.D{{Key: r.Meta.SoftDeleteField, Value: r.Meta.liveRecordCondition()}}
	if len(filter) == 0 {
		return sdFilter
	}
	return bson.D{{Key: "$and", Value: bson.A{filter, sdFilter}}}
}

// Count returns the number of documents matching the query's filters.
func (r *BaseCRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	prep, err := r.prepareListQuery(ctx, qarg...)
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
	prep, err := r.prepareListQuery(ctx, qarg...)
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

	entities, err := r.decodeList(ctx, cursor)
	if err != nil {
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
	if err := r.Meta.encodeWriteDoc(doc); err != nil {
		return entity, err
	}
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
	if err := r.Meta.encodeWriteValues(fieldNames, vals); err != nil {
		return entity, err
	}
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

	updated, err := r.getByFilter(ctx, filter)
	if err != nil {
		return entity, fmt.Errorf("mongo patch refetch: %w", err)
	}
	return updated, nil
}

// getByFilter fetches the single document matching filter into a fresh entity,
// applying value codecs on decode.
func (r *BaseCRUD[T, ID]) getByFilter(ctx context.Context, filter bson.D) (T, error) {
	var entity T
	if !r.Meta.HasCodecs() {
		err := r.Collection.FindOne(ctx, filter).Decode(&entity)
		return entity, err
	}
	var raw bson.M
	if err := r.Collection.FindOne(ctx, filter).Decode(&raw); err != nil {
		return entity, err
	}
	err := unmarshalWithCodecs(&r.Meta, raw, &entity)
	return entity, err
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
