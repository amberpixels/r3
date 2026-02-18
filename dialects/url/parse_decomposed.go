package r3url

import (
	"net/url"

	"github.com/amberpixels/r3"
)

// parseDecomposed parses decomposed URL parameters into an r3.Query.
// Each R3 component is read from its own URL parameter.
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

	// Pagination
	pagination, err := ParsePagination(values, cfg.ParamNames)
	if err != nil {
		return r3.Query{}, err
	}
	q.Pagination = pagination

	return q, nil
}
