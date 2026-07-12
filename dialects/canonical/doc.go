// Package canonical is the single source of the string vocabulary the
// serialization dialects (JSON, YAML, TOML, URL) all map through: operators
// ("eq", "ne", "gt", ...), sort directions ("asc", "desc"), nulls positions
// ("first", "last"), and aggregate functions. Add or rename a term here first so
// every dialect stays consistent. Data-store dialects (SQL, BSON) use their own
// target-specific representations and skip this package.
package canonical
