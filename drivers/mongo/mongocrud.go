package r3mongo

import (
	"github.com/amberpixels/r3"
	enginemongo "github.com/amberpixels/r3/engine/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoCRUD is a CRUD repository based on MongoDB via the official Go driver v2.
// It embeds enginemongo.BaseCRUD which provides the full r3.CRUD implementation.
type MongoCRUD[T any, ID comparable] struct {
	*enginemongo.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &MongoCRUD[any, any]{}

// NewMongoCRUD creates a new MongoDB-based CRUD repository from a *mongo.Collection.
// Models should use `bson` struct tags for field mapping.
func NewMongoCRUD[T any, ID comparable](coll *mongo.Collection) *MongoCRUD[T, ID] {
	return &MongoCRUD[T, ID]{
		BaseCRUD: enginemongo.NewBaseCRUD[T, ID](coll),
	}
}

// NewMongoCRUDFromDB creates a new MongoDB-based CRUD repository from a *mongo.Database.
// The collection name is derived automatically from the struct type T
// (e.g., User -> "users", CityTranslation -> "city_translations").
func NewMongoCRUDFromDB[T any, ID comparable](db *mongo.Database) *MongoCRUD[T, ID] {
	return &MongoCRUD[T, ID]{
		BaseCRUD: enginemongo.NewBaseCRUDFromDB[T, ID](db),
	}
}

// Raw returns the BaseRaw escape hatch for custom MongoDB operations.
func (r *MongoCRUD[T, ID]) Raw() *enginemongo.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// Collection returns the underlying *mongo.Collection for advanced usage.
func (r *MongoCRUD[T, ID]) Collection() *mongo.Collection { return r.BaseCRUD.Collection }

// DB returns the underlying *mongo.Database.
func (r *MongoCRUD[T, ID]) DB() *mongo.Database { return r.BaseCRUD.Collection.Database() }
