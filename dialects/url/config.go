package r3url

// Mode determines the parsing/formatting strategy for URL query parameters.
type Mode int

const (
	// ModeAuto tries unified mode first (if the "query" param is present),
	// and falls back to decomposed mode otherwise. This is the default.
	ModeAuto Mode = iota
	// ModeUnified only accepts a single JSON parameter containing the full query.
	ModeUnified
	// ModeDecomposed only accepts decomposed parameters (fields, filters, sort, page, page_size).
	ModeDecomposed
)

// SortFormat controls how sort parameters are parsed and formatted.
type SortFormat int

const (
	// SortFormatColonDir uses "field:direction" pairs separated by commas.
	// Example: "name:asc,age:desc"
	// Nulls position can be appended: "name:asc:nulls_last"
	SortFormatColonDir SortFormat = iota

	// SortFormatSignPrefix uses a "-" prefix for descending, no prefix for ascending.
	// Example: "name,-age"
	// This format does not support nulls position.
	SortFormatSignPrefix

	// SortFormatJSON parses the sort parameter as a JSON array of sort objects,
	// using the same schema as the r3json dialect.
	// Example: [{"field":"name","direction":"asc"}]
	SortFormatJSON
)

// ParamNames allows customizing the URL parameter names used during parsing and formatting.
type ParamNames struct {
	// Query is the parameter name for unified mode. Default: "query".
	Query string
	// Fields is the parameter name for the fields list. Default: "fields".
	Fields string
	// Filters is the parameter name for the filters JSON array. Default: "filters".
	Filters string
	// Sort is the parameter name for the sort specification. Default: "sort".
	Sort string
	// PageNum is the parameter name for the page number. Default: "page".
	PageNum string
	// PageSize is the parameter name for the page size. Default: "page_size".
	PageSize string
}

// DefaultParamNames returns parameter names with sensible defaults.
func DefaultParamNames() ParamNames {
	return ParamNames{
		Query:    "query",
		Fields:   "fields",
		Filters:  "filters",
		Sort:     "sort",
		PageNum:  "page",
		PageSize: "page_size",
	}
}

// SortConfig configures sort parsing and formatting.
type SortConfig struct {
	// Format controls how sort is parsed/formatted. Default: SortFormatColonDir.
	Format SortFormat
}

// FilterConfig configures filter parsing.
type FilterConfig struct {
	// AllowDjangoStyle enables parsing field__op=value query parameters as filters.
	// Default: false.
	AllowDjangoStyle bool

	// DjangoSeparator is the separator between field name and operator in Django-style params.
	// Default: "__" (double underscore).
	DjangoSeparator string

	// DjangoFields restricts which fields can be used in Django-style filters.
	// If empty and AllowDjangoStyle is true, all fields are allowed (validated for identifier safety).
	// Setting this is recommended as a security safeguard to prevent arbitrary query parameters
	// from being interpreted as filters.
	DjangoFields []string
}

// Config controls how the URL dialect parses and formats query parameters.
type Config struct {
	// Mode determines the parsing strategy. Default: ModeAuto.
	Mode Mode

	// ParamNames customizes the URL parameter names.
	ParamNames ParamNames

	// Sort configures sort parsing and formatting.
	Sort SortConfig

	// Filter configures filter parsing.
	Filter FilterConfig
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Mode:       ModeAuto,
		ParamNames: DefaultParamNames(),
		Sort: SortConfig{
			Format: SortFormatColonDir,
		},
		Filter: FilterConfig{
			AllowDjangoStyle: false,
			DjangoSeparator:  "__",
		},
	}
}

// Option is a functional option for configuring the URL dialect.
type Option func(*Config)

// resolveConfig applies functional options to the default config and returns the result.
func resolveConfig(opts ...Option) Config {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// WithMode sets the parsing/formatting mode.
func WithMode(m Mode) Option {
	return func(c *Config) {
		c.Mode = m
	}
}

// WithParamNames sets custom parameter names.
func WithParamNames(p ParamNames) Option {
	return func(c *Config) {
		c.ParamNames = p
	}
}

// WithSortFormat sets the sort format.
func WithSortFormat(f SortFormat) Option {
	return func(c *Config) {
		c.Sort.Format = f
	}
}

// WithDjangoStyleFilters enables Django-style filter parsing for the specified fields.
// If no fields are provided, all fields are allowed (not recommended for security reasons).
func WithDjangoStyleFilters(fields ...string) Option {
	return func(c *Config) {
		c.Filter.AllowDjangoStyle = true
		c.Filter.DjangoFields = fields
	}
}

// WithDjangoSeparator sets the separator used in Django-style filter parameters.
// Default is "__" (double underscore).
func WithDjangoSeparator(sep string) Option {
	return func(c *Config) {
		c.Filter.DjangoSeparator = sep
	}
}

// reservedParamNames returns the set of parameter names that are used by the dialect itself,
// so they are not considered as Django-style filter candidates.
func (c *Config) reservedParamNames() map[string]struct{} {
	return map[string]struct{}{
		c.ParamNames.Query:    {},
		c.ParamNames.Fields:   {},
		c.ParamNames.Filters:  {},
		c.ParamNames.Sort:     {},
		c.ParamNames.PageNum:  {},
		c.ParamNames.PageSize: {},
	}
}
