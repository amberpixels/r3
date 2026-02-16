// Package r3toml provides bidirectional conversion between r3 query types and TOML.
//
// Category: Serialization dialect.
//
// It serializes [r3.Filters], [r3.Sorts], [r3.PaginationSpec], and [r3.Fields]
// to TOML for configuration files, and parses them back. Like the YAML dialect,
// the TOML dialect uses human-readable names (field/operator/value).
//
// Limitation: TOML has no null type. Filters with nil values (IS NULL checks)
// cannot be encoded to TOML. FilterToTOML returns an error for such filters.
package r3toml
