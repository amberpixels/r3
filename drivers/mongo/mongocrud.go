// Package r3mongo provides an r3.CRUD[T, ID] driver backed by the official MongoDB Go driver v2.
//
// Driver: go.mongodb.org/mongo-driver/v2
// Source: https://github.com/mongodb/mongo-go-driver
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Patch, Delete)
//   - Filters, Sorts, Pagination via the r3 BSON dialect
//   - Field selection (projection)
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Soft-delete support (via r3:"soft_delete" struct tag)
//   - Relation preloading (has-many, belongs-to) via separate queries
//   - Raw escape hatch (BaseRaw) for arbitrary MongoDB operations
//
// Model structs should use `bson` struct tags for field mapping, or `r3` tags.
// The primary key field defaults to `_id`. Use `r3:"pk"` or `bson:"_id"` to customize.
//
// Example:
//
//	type User struct {
//	    ID   primitive.ObjectID `bson:"_id,omitempty" r3:"pk"`
//	    Name string             `bson:"name"`
//	    Age  int                `bson:"age"`
//	}
//
//	client, _ := mongo.Connect(options.Client().ApplyURI("mongodb://localhost:27017"))
//	db := client.Database("mydb")
//	userRepo := r3mongo.NewMongoCRUD[User, primitive.ObjectID](db.Collection("users"))
//
//	// Or let the collection name be derived from the struct name:
//	userRepo := r3mongo.NewMongoCRUDFromDB[User, primitive.ObjectID](db)
package r3mongo

import (
	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/mongobase"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// MongoCRUD is a CRUD repository based on MongoDB via the official Go driver v2.
// It embeds mongobase.BaseCRUD which provides the full r3.CRUD implementation.
type MongoCRUD[T any, ID comparable] struct {
	*mongobase.BaseCRUD[T, ID]
}

var _ r3.CRUD[any, any] = &MongoCRUD[any, any]{}

// NewMongoCRUD creates a new MongoDB-based CRUD repository from a *mongo.Collection.
// Models should use `bson` struct tags for field mapping.
func NewMongoCRUD[T any, ID comparable](coll *mongo.Collection) *MongoCRUD[T, ID] {
	return &MongoCRUD[T, ID]{
		BaseCRUD: mongobase.NewBaseCRUD[T, ID](coll),
	}
}

// NewMongoCRUDFromDB creates a new MongoDB-based CRUD repository from a *mongo.Database.
// The collection name is derived automatically from the struct type T
// (e.g., User -> "users", CityTranslation -> "city_translations").
func NewMongoCRUDFromDB[T any, ID comparable](db *mongo.Database) *MongoCRUD[T, ID] {
	return &MongoCRUD[T, ID]{
		BaseCRUD: mongobase.NewBaseCRUDFromDB[T, ID](db),
	}
}

// Raw returns the BaseRaw escape hatch for custom MongoDB operations.
func (r *MongoCRUD[T, ID]) Raw() *mongobase.BaseRaw[T, ID] { return r.BaseCRUD.Raw }

// Collection returns the underlying *mongo.Collection for advanced usage.
func (r *MongoCRUD[T, ID]) Collection() *mongo.Collection { return r.BaseCRUD.Collection }

// DB returns the underlying *mongo.Database.
func (r *MongoCRUD[T, ID]) DB() *mongo.Database { return r.BaseCRUD.Collection.Database() }
