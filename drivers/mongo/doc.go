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
//   - Restore and HardDelete via the features/softdelete SoftDeleter interface
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
