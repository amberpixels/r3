package r3mongo

import (
	"github.com/amberpixels/r3"
	enginemongo "github.com/amberpixels/r3/engine/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoCRUD is a MongoDB repository; enginemongo.BaseCRUD supplies the r3.CRUD impl.
type MongoCRUD[T any, ID comparable] struct {
	*enginemongo.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &MongoCRUD[any, any]{}

// NewMongoCRUD builds a repository from a collection. Map fields with `bson` tags.
func NewMongoCRUD[T any, ID comparable](coll *mongo.Collection, opts ...r3.Option) *MongoCRUD[T, ID] {
	return &MongoCRUD[T, ID]{
		BaseCRUD: enginemongo.NewBaseCRUD[T, ID](coll, opts...),
	}
}

// NewMongoCRUDFromDB builds a repository from a database, deriving the collection
// name from T (e.g. User -> "users", CityTranslation -> "city_translations").
func NewMongoCRUDFromDB[T any, ID comparable](db *mongo.Database, opts ...r3.Option) *MongoCRUD[T, ID] {
	return &MongoCRUD[T, ID]{
		BaseCRUD: enginemongo.NewBaseCRUDFromDB[T, ID](db, opts...),
	}
}

// NewMongoQuerier builds a read-only repository from a collection ([r3.Querier]
// enforces it).
func NewMongoQuerier[T any, ID comparable](coll *mongo.Collection, opts ...r3.Option) r3.Querier[T, ID] {
	return NewMongoCRUD[T, ID](coll, opts...)
}

// NewMongoQuerierFromDB builds a read-only repository from a database, deriving the
// collection name from T ([r3.Querier] enforces read-only).
func NewMongoQuerierFromDB[T any, ID comparable](db *mongo.Database, opts ...r3.Option) r3.Querier[T, ID] {
	return NewMongoCRUDFromDB[T, ID](db, opts...)
}

// Raw returns the BaseRaw escape hatch for custom MongoDB operations.
func (r *MongoCRUD[T, ID]) Raw() *enginemongo.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// Collection returns the underlying *mongo.Collection for advanced usage.
func (r *MongoCRUD[T, ID]) Collection() *mongo.Collection { return r.BaseCRUD.Collection }

// DB returns the underlying *mongo.Database.
func (r *MongoCRUD[T, ID]) DB() *mongo.Database { return r.BaseCRUD.Collection.Database() }
