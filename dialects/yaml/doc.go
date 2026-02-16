// Package r3yaml provides bidirectional conversion between r3 query types and YAML.
//
// Category: Serialization dialect.
//
// It serializes [r3.Filters], [r3.Sorts], [r3.PaginationSpec], and [r3.Fields]
// to YAML for configuration files, and parses them back. Unlike the JSON dialect
// which uses compact field names (f/op/v), the YAML dialect uses human-readable
// names (field/operator/value) suitable for configuration files.
package r3yaml
