package r3

// Config holds framework-level configuration (naming, default query behavior)
// shared across models. Build it with [DefaultConfig], override fields, then pass
// it to a constructor via [WithConfig]. Treat as read-only after construction.
type Config struct {
	// Naming maps well-known fields to storage column names.
	Naming NamingConfig

	// Defaults controls default query behavior (e.g. page size).
	Defaults DefaultsConfig
}

// NamingConfig maps well-known fields to storage column names. Empty means
// "use the default".
type NamingConfig struct {
	// CreatedAtField is the creation-timestamp column. Default: "created_at".
	CreatedAtField string

	// UpdatedAtField is the update-timestamp column. Default: "updated_at".
	UpdatedAtField string

	// DeletedAtField is the soft-delete-timestamp column. Default: "deleted_at".
	DeletedAtField string
}

// DefaultsConfig controls default query behavior.
type DefaultsConfig struct {
	// PageSize is the default page size when pagination is active but no explicit
	// size is given. Default: [PageSizeDefault].
	PageSize int

	// Unpaginated, when true, makes List return ALL matching rows by default (no
	// page-size cap); a per-query Query.Pagination still opts back in. Takes
	// precedence over PageSize.
	//
	// Use with care on large tables; prefer the per-query [Unpaginated] escape
	// hatch when only some call sites need everything.
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
