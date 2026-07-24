package r3mongo

import (
	"go.mongodb.org/mongo-driver/v2/mongo"

	enginemongo "github.com/amberpixels/r3/engine/mongo"
)

// MongoRaw wraps enginemongo.BaseRaw.
//
// Deprecated: use enginemongo.BaseRaw directly via MongoCRUD.Raw().
type MongoRaw[T any, ID any] struct {
	*enginemongo.BaseRaw[T, ID]
}

// NewMongoRaw creates a new MongoRaw instance.
func NewMongoRaw[T any, ID comparable](coll *mongo.Collection) *MongoRaw[T, ID] {
	meta := enginemongo.GetStructMeta[T]()
	return &MongoRaw[T, ID]{
		BaseRaw: enginemongo.NewBaseRaw[T, ID](coll, meta),
	}
}
