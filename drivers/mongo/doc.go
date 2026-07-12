// Package r3mongo is an r3.CRUD[T, ID] driver backed by the official MongoDB Go
// driver v2 (go.mongodb.org/mongo-driver/v2). Filters/sorts/pagination go through
// the r3 BSON dialect; it supports field projection, soft-delete (via
// r3:"soft_delete", plus Restore/HardDelete), and has-many/belongs-to preloads run
// as separate queries.
//
// Map fields with `bson` (or `r3`) tags; the PK defaults to `_id`, overridable with
// `r3:"pk"` or `bson:"_id"`.
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
