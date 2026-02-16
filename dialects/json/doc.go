// Package r3json provides bidirectional conversion between r3 query types and JSON.
//
// Category: Serialization dialect.
//
// It serializes [r3.Filters], [r3.Sorts], [r3.PaginationSpec], and [r3.Fields]
// to JSON for REST API request/response bodies, and parses them back.
// This is useful for building HTTP APIs on top of r3 repositories.
package r3json
