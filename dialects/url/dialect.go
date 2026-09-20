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

	q, err := parseByMode(values, cfg)
	if err != nil {
		return r3.Query{}, err
	}
	// A request naming both projection forms is a conflict, not a precedence for
	// this dialect to invent. Defer to the rule the engines enforce, so the two
	// can never disagree about what a conflict is.
	if err := q.ValidateProjection(); err != nil {
		return r3.Query{}, newError(err)
	}
	return q, nil
}

// parseByMode dispatches to the configured mode's parser. ModeAuto reads the
// request as unified whenever the unified param is present.
func parseByMode(values url.Values, cfg Config) (r3.Query, error) {
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

	// Same rule as ParseQuery, or this would emit a URL the dialect refuses to
	// read back.
	if err := q.ValidateProjection(); err != nil {
		return nil, newError(err)
	}

	switch cfg.Mode {
	case ModeUnified:
		return formatUnified(q, cfg)
	case ModeDecomposed:
		return formatDecomposed(q, cfg)
	default: // ModeAuto -> defaults to decomposed for formatting
		return formatDecomposed(q, cfg)
	}
}
