// Package enginemongo provides a shared base driver for all r3 CRUD implementations
// backed by MongoDB. It contains reflection-based struct metadata, BSON query building
// helpers, and BaseCRUD / BaseRaw generic types.
//
// This is the MongoDB equivalent of the enginesql package.
//
// Driver-specific packages (e.g. drivers/mongo) embed BaseCRUD and only override
// behavior that differs.
package enginemongo
