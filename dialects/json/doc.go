// Package r3json converts r3 query types ([r3.Filters], [r3.Sorts],
// [r3.PaginationSpec], [r3.Fields]) to and from JSON for REST request/response
// bodies. Filters use the compact form {"f":field,"op":operator,"v":value}.
//
// Category: Serialization dialect.
package r3json
