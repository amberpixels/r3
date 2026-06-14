package r3

// Config holds framework-level configuration for R3 repositories.
// It controls naming conventions, default query behavior, and other
// cross-cutting concerns that are not specific to any single model.
//
// Use [DefaultConfig] to get a Config with sensible defaults,
// then override individual fields as needed.
//
// Config is intended to be read-only after construction. Pass it to
// engine/driver constructors via [WithConfig].
type Config struct {
	// Naming controls how R3 maps well-known fields to storage column names.
	Naming NamingConfig

	// Defaults controls default query behavior (e.g. page size).
	Defaults DefaultsConfig
}

// NamingConfig controls how R3 maps well-known fields to storage column names.
// Empty strings mean "use the default".
type NamingConfig struct {
	// CreatedAtField is the storage column name for creation timestamps.
	// Default: "created_at"
	CreatedAtField string

	// UpdatedAtField is the storage column name for update timestamps.
	// Default: "updated_at"
	UpdatedAtField string

	// DeletedAtField is the storage column name for soft-delete timestamps.
	// Default: "deleted_at"
	DeletedAtField string
}

// DefaultsConfig controls default query behavior.
type DefaultsConfig struct {
	// PageSize is the default number of items per page when pagination
	// is active but no explicit page size is provided.
	// Default: 100 (same as PageSizeDefault)
	PageSize int

	// Unpaginated, when true, makes List return ALL matching rows by default —
	// no implicit page-size cap. Individual queries can still opt back into
	// pagination per call by setting Query.Pagination. Takes precedence over
	// PageSize.
	//
	// Use with care on large tables; prefer the per-query r3.Unpaginated()
	// escape hatch when only some call sites need everything.
	Unpaginated bool
}

// DefaultConfig returns a Config with all defaults applied.
func DefaultConfig() Config {
	return Config{
		Naming: NamingConfig{
			CreatedAtField: "created_at",
			UpdatedAtField: "updated_at",
			DeletedAtField: "deleted_at",
		},
		Defaults: DefaultsConfig{
			PageSize: PageSizeDefault,
		},
	}
}
