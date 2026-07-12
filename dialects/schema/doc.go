// Package r3schema serializes an [r3.Schema] to a stable, versioned, public-only
// JSON shape for introspection.
//
// Category: Serialization dialect.
//
// A consumer can expose the result (e.g. at a "/-/schema" endpoint) so a
// frontend builds column pickers and a dynamic "+ filter" UI from the declared
// attributes.
//
// Only the public projection is emitted: non-Queryable attributes are omitted,
// so the system/worker bypass is never advertised. The reserved "computed" flag
// ships from day one to keep the contract forward-compatible.
package r3schema
