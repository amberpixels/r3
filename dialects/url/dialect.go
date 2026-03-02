package r3url

import (
	"net/url"

	"github.com/amberpixels/r3"
)

// ParseQuery parses url.Values into an r3.Query using the configured mode and options.
//
// In ModeAuto (default), it checks for the unified "query" parameter first.
// If present, unified mode is used; otherwise, decomposed mode is used.
//
// Example usage:
//
//	// Auto mode (default):
//	q, err := r3url.ParseQuery(r.URL.Query())
//
//	// Decomposed with sign-prefix sort:
//	q, err := r3url.ParseQuery(r.URL.Query(),
//	    r3url.WithMode(r3url.ModeDecomposed),
//	    r3url.WithSortFormat(r3url.SortFormatSignPrefix),
//	)
//
//	// With Django-style filters:
//	q, err := r3url.ParseQuery(r.URL.Query(),
//	    r3url.WithDjangoStyleFilters("status", "age", "name"),
//	)
func ParseQuery(values url.Values, opts ...Option) (r3.Query, error) {
	cfg := resolveConfig(opts...)

	//nolint:exhaustive // ModeAuto is the default case
	switch cfg.Mode {
	case ModeUnified:
		return parseUnified(values, cfg)
	case ModeDecomposed:
		return parseDecomposed(values, cfg)
	default: // ModeAuto
		if values.Get(cfg.ParamNames.Query) != "" {
			return parseUnified(values, cfg)
		}
		return parseDecomposed(values, cfg)
	}
}

// FormatQuery formats an r3.Query into url.Values using the configured mode and options.
//
// In ModeAuto, it defaults to decomposed mode for formatting (since decomposed
// produces more human-readable URLs). Use WithMode(ModeUnified) to format as
// a single JSON query parameter instead.
//
// Example usage:
//
//	values, err := r3url.FormatQuery(q)
//	// values.Encode() -> "fields=id,name&sort=name:asc&page=1&page_size=25"
//
//	values, err := r3url.FormatQuery(q, r3url.WithMode(r3url.ModeUnified))
//	// values.Encode() -> "query={...json...}"
func FormatQuery(q r3.Query, opts ...Option) (url.Values, error) {
	cfg := resolveConfig(opts...)

	//nolint:exhaustive // ModeAuto is the default case
	switch cfg.Mode {
	case ModeUnified:
		return formatUnified(q, cfg)
	case ModeDecomposed:
		return formatDecomposed(q, cfg)
	default: // ModeAuto -> defaults to decomposed for formatting
		return formatDecomposed(q, cfg)
	}
}
