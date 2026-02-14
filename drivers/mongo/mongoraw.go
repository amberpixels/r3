package r3mongo

import (
	"github.com/amberpixels/r3/mongobase"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoRaw is a thin wrapper around mongobase.BaseRaw for backward compatibility.
//
// Deprecated: Use mongobase.BaseRaw directly via MongoCRUD.Raw().
type MongoRaw[T any, ID any] struct {
	*mongobase.BaseRaw[T, ID]
}

// NewMongoRaw creates a new MongoRaw instance.
func NewMongoRaw[T any, ID comparable](coll *mongo.Collection) *MongoRaw[T, ID] {
	meta := mongobase.GetStructMeta[T]()
	return &MongoRaw[T, ID]{
		BaseRaw: mongobase.NewBaseRaw[T, ID](coll, meta),
	}
}
