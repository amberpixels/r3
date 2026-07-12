// Package r3toml converts r3 query types ([r3.Filters], [r3.Sorts],
// [r3.PaginationSpec], [r3.Fields]) to and from TOML for configuration files,
// using human-readable names (field/operator/value) like the YAML dialect.
//
// Limitation: TOML has no null type. Filters with nil values (IS NULL checks)
// cannot be encoded to TOML. FilterToTOML returns an error for such filters.
package r3toml
