// Package enginemongo is the MongoDB engine for r3: a full r3.CRUD backed by the
// official Go driver v2, using reflection-based struct metadata and BSON query
// building. The drivers/mongo package wraps it as a thin constructor.
//
// Key types:
//   - [BaseCRUD]: full r3.CRUD backed by *mongo.Collection
//   - [BaseRaw]: escape hatch for arbitrary MongoDB operations
//   - [StructMeta]: reflection-based struct metadata (fields, PK, relations, soft-delete)
//   - [PreparedListQuery]: pre-computed BSON filter/sort/pagination from an r3.Query
//
// Field mapping reads `bson` or `r3` struct tags; the primary key defaults to `_id`.
package enginemongo
