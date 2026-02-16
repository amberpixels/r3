// Package canonical provides shared parse and format functions for the
// canonical string representations of r3 query components.
//
// All serialization dialects (JSON, YAML, TOML) use the same canonical
// strings for operators ("eq", "ne", "gt", ...), sort directions ("asc", "desc"),
// and nulls positions ("first", "last"). This package centralizes that mapping.
//
// Data store dialects (SQL, BSON) typically do not use this package as they
// have their own target-specific representations.
package canonical
