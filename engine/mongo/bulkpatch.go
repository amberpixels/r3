package enginemongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/amberpixels/r3"
	r3bson "github.com/amberpixels/r3/dialects/bson"
)

var _ r3.BulkPatcher[any, any] = &BaseCRUD[any, any]{}

// PatchWhere sets fields (taken from entity's values) on every document matching
// filters and returns the modified-document count - the multi-document Patch. It
// shares Patch's field validation (ValidatePatchFields: no unknown, ID, or
// soft-delete field), maps to a single UpdateMany with a $set, and excludes
// soft-deleted documents by default. Relationship ("has") filters are not
// supported and return an error. Mirrors engine/sql/bulkpatch.go.
func (r *BaseCRUD[T, ID]) PatchWhere(
	ctx context.Context, filters r3.Filters, entity T, fields r3.Fields,
) (int64, error) {
	fieldNames, err := r.Meta.ValidatePatchFields(r3.FieldsToStrings(fields))
	if err != nil {
		return 0, err
	}

	if hasRelationFilter(filters) {
		return 0, errors.New("r3/engine/mongo: PatchWhere does not support relationship filters")
	}

	// Encode codec'd filter args to stored form before conversion.
	filters, err = r3.EncodeFilterCodecs(r.CodecSchema, filters)
	if err != nil {
		return 0, fmt.Errorf("mongo bulk patch: encode filter codecs: %w", err)
	}
	userFilter, err := r3bson.FiltersToBSON(filters)
	if err != nil {
		return 0, fmt.Errorf("mongo bulk patch: convert filters: %w", err)
	}
	filter := r.applyLiveFilter(userFilter, false)

	setDoc := make(bson.D, 0, len(fieldNames))
	vals := r.Meta.FieldValuesForFields(entity, fieldNames)
	if err := r.Meta.encodeWriteValues(fieldNames, vals); err != nil {
		return 0, err
	}
	for i, name := range fieldNames {
		setDoc = append(setDoc, bson.E{Key: name, Value: vals[i]})
	}
	update := bson.D{{Key: setOp, Value: setDoc}}

	res, err := r.Collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("mongo bulk patch: %w", err)
	}
	return res.ModifiedCount, nil
}
