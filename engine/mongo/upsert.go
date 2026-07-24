package enginemongo

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/amberpixels/r3"
)

var _ r3.Upserter[any, any] = &BaseCRUD[any, any]{}

// setOnInsertOp is the MongoDB update operator applying values only when the
// upsert takes the insert branch.
const setOnInsertOp = "$setOnInsert"

// Upsert inserts entity, or updates the colliding document on a conflict against
// the conflict target (the _id primary key by default), via UpdateOne with
// upsert:true.
//
// The conflict target is UpsertSpec.ConflictColumns (a unique-index field set),
// defaulting to the PK. UpsertSpec.UpdateFields, when set, restricts the fields
// overwritten on conflict (Patch-validated); empty means a full replace of every
// non-conflict field. Fields not written by the update branch are still applied
// on insert via $setOnInsert, so a new document is stored complete. The stored
// document is re-fetched and returned. Mirrors the SQL semantics in
// engine/sql/upsert.go.
func (r *BaseCRUD[T, ID]) Upsert(ctx context.Context, entity T, opts ...r3.UpsertOption) (T, error) {
	spec := r3.NewUpsertSpec(opts...)

	conflictCols := spec.ConflictColumns
	if len(conflictCols) == 0 {
		conflictCols = []string{r.Meta.IDField}
	}

	// Upsert keyed on the default PK with a zero _id has nothing to conflict with
	// (and filtering by a zero _id would collide every such upsert onto one
	// document), so it is a plain insert - as a serial PK is DB-generated in SQL.
	if len(conflictCols) == 1 && conflictCols[0] == r.Meta.IDField && r.idIsZero(entity) {
		return r.Create(ctx, entity)
	}

	conflictSet := make(map[string]bool, len(conflictCols))
	filter := make(bson.D, 0, len(conflictCols))
	conflictVals := r.Meta.FieldValuesForFields(entity, conflictCols)
	if err := r.Meta.encodeWriteValues(conflictCols, conflictVals); err != nil {
		return entity, err
	}
	for i, c := range conflictCols {
		conflictSet[c] = true
		filter = append(filter, bson.E{Key: c, Value: conflictVals[i]})
	}

	updateFields, err := r.upsertUpdateFields(spec, conflictSet)
	if err != nil {
		return entity, err
	}
	updateSet := make(map[string]bool, len(updateFields))
	for _, f := range updateFields {
		updateSet[f] = true
	}

	setDoc := make(bson.D, 0, len(updateFields))
	vals := r.Meta.FieldValuesForFields(entity, updateFields)
	if err := r.Meta.encodeWriteValues(updateFields, vals); err != nil {
		return entity, err
	}
	for i, name := range updateFields {
		setDoc = append(setDoc, bson.E{Key: name, Value: vals[i]})
	}

	// $setOnInsert covers the fields the update branch does not write, so an
	// inserted document is complete. The conflict columns come from the filter's
	// equality on insert, and a zero _id is left out so Mongo generates one.
	setOnInsert, err := r.upsertSetOnInsert(entity, conflictSet, updateSet)
	if err != nil {
		return entity, err
	}

	update := bson.D{}
	if len(setDoc) > 0 {
		update = append(update, bson.E{Key: setOp, Value: setDoc})
	}
	if len(setOnInsert) > 0 {
		update = append(update, bson.E{Key: setOnInsertOp, Value: setOnInsert})
	}
	if len(update) == 0 {
		return entity, errors.New("r3/engine/mongo: upsert has no fields to write")
	}

	res, err := r.Collection.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return entity, fmt.Errorf("mongo upsert: %w", err)
	}

	fetchFilter := filter
	if res.UpsertedID != nil {
		fetchFilter = bson.D{{Key: r.Meta.IDField, Value: res.UpsertedID}}
	}

	stored, err := r.getByFilter(ctx, fetchFilter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return entity, r3.ErrNotFound
		}
		return entity, fmt.Errorf("mongo upsert refetch: %w", err)
	}
	return stored, nil
}

// upsertUpdateFields returns the BSON field names the on-conflict update branch
// writes: the Patch-validated UpdateFields when set, else every non-ID,
// non-conflict field (a full replace).
func (r *BaseCRUD[T, ID]) upsertUpdateFields(spec r3.UpsertSpec, conflictSet map[string]bool) ([]string, error) {
	if len(spec.UpdateFields) > 0 {
		fields, err := r.Meta.ValidatePatchFields(r3.FieldsToStrings(spec.UpdateFields))
		if err != nil {
			return nil, err
		}
		return fields, nil
	}

	var fields []string
	for _, f := range r.Meta.Fields {
		if f == r.Meta.IDField || conflictSet[f] {
			continue
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// upsertSetOnInsert builds the $setOnInsert document: every field not already
// covered by the conflict filter or the $set update, so an inserted document is
// complete. A zero _id is omitted so Mongo generates one. Codec'd fields are
// encoded to stored form.
func (r *BaseCRUD[T, ID]) upsertSetOnInsert(entity T, conflictSet, updateSet map[string]bool) (bson.D, error) {
	doc := bson.D{}
	all := r.Meta.FieldValues(entity)
	if err := r.Meta.encodeWriteValues(r.Meta.Fields, all); err != nil {
		return nil, err
	}
	for i, name := range r.Meta.Fields {
		if conflictSet[name] || updateSet[name] {
			continue
		}
		if name == r.Meta.IDField && isZeroValue(all[i]) {
			continue
		}
		doc = append(doc, bson.E{Key: name, Value: all[i]})
	}
	return doc, nil
}

// idIsZero reports whether the entity's _id holds its type's zero value.
func (r *BaseCRUD[T, ID]) idIsZero(entity T) bool {
	return isZeroValue(r.Meta.IDValue(entity))
}

// isZeroValue reports whether v is nil or the zero value of its type.
func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	return reflect.ValueOf(v).IsZero()
}
