package r3url

import (
	"net/url"

	"github.com/amberpixels/r3"
)

// ParseQuery parses url.Values into an r3.Query using the configured [Mode] and options.
//
//	q, err := r3url.ParseQuery(r.URL.Query(),
//	    r3url.WithMode(r3url.ModeDecomposed),
//	    r3url.WithSortFormat(r3url.SortFormatSignPrefix),
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

// FormatQuery formats an r3.Query into url.Values using the configured [Mode] and options.
// ModeAuto formats as decomposed (more readable URLs); use [WithMode]([ModeUnified]) for
// a single JSON "query" param.
//
//	values, err := r3url.FormatQuery(q)
//	// values.Encode() -> "fields=id,name&sort=name:asc&page=1&page_size=25"
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
