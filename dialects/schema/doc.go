// Package r3schema serializes an [r3.Schema] to a stable, versioned, public-only
// JSON shape for introspection.
//
// Category: Serialization dialect.
//
// A consumer can expose the result (e.g. at a "/-/schema" endpoint) so a
// frontend can build column pickers and a dynamic "+ filter" UI from the
// declared attributes. The serialization is pure and stateless — it has no I/O
// and no state, matching the dialect contract.
//
// Only the public projection is emitted: attributes without the Queryable
// capability are omitted, and the system/worker bypass is never advertised. The
// reserved "computed" flag is included from day one so the contract is
// forward-compatible.
package r3schema
