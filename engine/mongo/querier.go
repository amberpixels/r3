package enginemongo

import (
	"github.com/amberpixels/r3"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// NewBaseQuerier creates a read-only repository backed by MongoDB.
// It returns [r3.Querier], providing a compile-time guarantee that the
// caller cannot perform write operations.
//
// Internally this constructs a full BaseCRUD but only exposes the
// Querier interface (Get, List).
func NewBaseQuerier[T any, ID comparable](coll *mongo.Collection, opts ...r3.Option) r3.Querier[T, ID] {
	return NewBaseCRUD[T, ID](coll, opts...)
}

// NewBaseQuerierFromDB creates a read-only repository backed by MongoDB,
// deriving the collection name from the struct type T.
func NewBaseQuerierFromDB[T any, ID comparable](db *mongo.Database, opts ...r3.Option) r3.Querier[T, ID] {
	return NewBaseCRUDFromDB[T, ID](db, opts...)
}
