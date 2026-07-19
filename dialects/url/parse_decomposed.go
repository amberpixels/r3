package r3url

import (
	"net/url"

	"github.com/amberpixels/r3"
	r3when "github.com/amberpixels/r3/dialects/when"
)

// parseDecomposed reads each r3 component from its own URL parameter into an r3.Query.
func parseDecomposed(values url.Values, cfg Config) (r3.Query, error) {
	q := r3.NewQuery()

	// Fields
	rawFields := values.Get(cfg.ParamNames.Fields)
	if rawFields != "" {
		fields, err := ParseFields(rawFields)
		if err != nil {
			return r3.Query{}, err
		}
		q.Fields = fields
	}

	// Filters (JSON array)
	rawFilters := values.Get(cfg.ParamNames.Filters)
	if rawFilters != "" {
		filters, err := ParseFilters(rawFilters)
		if err != nil {
			return r3.Query{}, err
		}
		q.Filters = filters
	}

	// Django-style filters (if enabled)
	if cfg.Filter.AllowDjangoStyle {
		djangoFilters, err := ParseDjangoFilters(values, cfg)
		if err != nil {
			return r3.Query{}, err
		}
		if len(djangoFilters) > 0 {
			q.Filters = q.Filters.MergeWith(djangoFilters)
		}
	}

	// Sort
	rawSort := values.Get(cfg.ParamNames.Sort)
	if rawSort != "" {
		sorts, err := ParseSort(rawSort, cfg.Sort.Format)
		if err != nil {
			return r3.Query{}, err
		}
		q.Sorts = sorts
	}

	// Cursor pagination (takes precedence over offset-based)
	cursor, err := ParseCursorPagination(values, cfg.ParamNames)
	if err != nil {
		return r3.Query{}, err
	}
	if cursor != nil {
		q.Cursor = cursor
	} else {
		// Offset-based pagination
		pagination, err := ParsePagination(values, cfg.ParamNames)
		if err != nil {
			return r3.Query{}, err
		}
		q.Pagination = pagination
	}

	return applyWhenFilter(values, cfg, q)
}

// applyWhenFilter merges the ?when= recurring time-pattern filter into q when the
// dialect has opted in via [WithWhenFilter]. It runs in every parsing mode so an
// opted-in caller gets the filter whether the request is decomposed or unified -
// otherwise ModeAuto would silently drop ?when= whenever a ?query= is also
// present. Unlike a Django suffix, this param only exists because the app opted
// in, so an unknown keyword is a client error: the years message (listing valid
// terms) is passed through verbatim.
func applyWhenFilter(values url.Values, cfg Config, q r3.Query) (r3.Query, error) {
	if !cfg.Filter.AllowWhen {
		return q, nil
	}
	raw := values.Get(cfg.ParamNames.When)
	if raw == "" {
		return q, nil
	}
	whenFilters, err := r3when.Parse(cfg.Filter.WhenField, raw)
	if err != nil {
		return r3.Query{}, err
	}
	if len(whenFilters) > 0 {
		q.Filters = q.Filters.MergeWith(whenFilters)
	}
	return q, nil
}
