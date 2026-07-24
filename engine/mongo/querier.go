package enginemongo

import (
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/amberpixels/r3"
)

// NewBaseQuerier returns a [BaseCRUD] as a read-only [r3.Querier], so callers get
// a compile-time guarantee they cannot write.
func NewBaseQuerier[T any, ID comparable](coll *mongo.Collection, opts ...r3.Option) r3.Querier[T, ID] {
	return NewBaseCRUD[T, ID](coll, opts...)
}

// NewBaseQuerierFromDB is [NewBaseQuerier], deriving the collection name from T.
func NewBaseQuerierFromDB[T any, ID comparable](db *mongo.Database, opts ...r3.Option) r3.Querier[T, ID] {
	return NewBaseCRUDFromDB[T, ID](db, opts...)
}
