// Package enginemongo provides the MongoDB engine for r3.
//
// It implements the full r3.CRUD interface using the official MongoDB Go driver v2,
// with reflection-based struct metadata and BSON query building.
// The drivers/mongo package wraps this engine as a thin constructor.
//
// Key types:
//   - [BaseCRUD]: full r3.CRUD backed by *mongo.Collection
//   - [BaseRaw]: escape hatch for arbitrary MongoDB operations
//   - [StructMeta]: reflection-based struct metadata (fields, PK, relations, soft-delete)
//   - [PreparedListQuery]: pre-computed BSON filter/sort/pagination from an r3.Query
//
// Model structs should use `bson` struct tags for field mapping, or `r3` tags.
// The primary key field defaults to `_id`.
package enginemongo
