package r3url

import (
	"net/url"

	"github.com/amberpixels/r3"
)

// formatDecomposed formats an r3.Query as decomposed URL parameters.
func formatDecomposed(q r3.Query, cfg Config) (url.Values, error) {
	values := make(url.Values)

	// Fields
	if len(q.Fields) > 0 {
		values.Set(cfg.ParamNames.Fields, FormatFields(q.Fields))
	}

	// Filters
	// Separate simple filters (for Django-style) from complex ones (for JSON)
	if len(q.Filters) > 0 {
		if cfg.Filter.AllowDjangoStyle {
			var jsonFilters r3.Filters
			var djangoFilters r3.Filters

			for _, f := range q.Filters {
				if isDjangoCompatible(f, cfg) {
					djangoFilters = append(djangoFilters, f)
				} else {
					jsonFilters = append(jsonFilters, f)
				}
			}

			// Format Django-style filters as individual params
			if len(djangoFilters) > 0 {
				djangoValues := FormatDjangoFilters(djangoFilters, cfg)
				for k, vs := range djangoValues {
					for _, v := range vs {
						values.Set(k, v)
					}
				}
			}

			// Format remaining complex filters as JSON
			if len(jsonFilters) > 0 {
				filtersStr, err := FormatFilters(jsonFilters)
				if err != nil {
					return nil, err
				}
				values.Set(cfg.ParamNames.Filters, filtersStr)
			}
		} else {
			filtersStr, err := FormatFilters(q.Filters)
			if err != nil {
				return nil, err
			}
			values.Set(cfg.ParamNames.Filters, filtersStr)
		}
	}

	// Sort
	if len(q.Sorts) > 0 {
		sortStr, err := FormatSort(q.Sorts, cfg.Sort.Format)
		if err != nil {
			return nil, err
		}
		if sortStr != "" {
			values.Set(cfg.ParamNames.Sort, sortStr)
		}
	}

	// Pagination
	if q.Pagination != nil {
		paginationValues := FormatPagination(q.Pagination, cfg.ParamNames)
		for k, vs := range paginationValues {
			for _, v := range vs {
				values.Set(k, v)
			}
		}
	}

	return values, nil
}

// isDjangoCompatible checks if a filter can be represented in Django-style format.
// Only simple field-operator-value filters (no AND/OR groups) qualify.
func isDjangoCompatible(f *r3.FilterSpec, cfg Config) bool {
	if f == nil || f.Field == nil {
		return false
	}
	if len(f.And) > 0 || len(f.Or) > 0 {
		return false
	}

	// If DjangoFields whitelist is set, check if the field is allowed
	if len(cfg.Filter.DjangoFields) > 0 {
		allowed := buildAllowedFieldsMap(cfg.Filter.DjangoFields)
		if _, ok := allowed[f.Field.String()]; !ok {
			return false
		}
	}

	return true
}
