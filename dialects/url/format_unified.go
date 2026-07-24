package r3url

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

// formatUnified formats an r3.Query as a unified ?query={...} parameter.
func formatUnified(q r3.Query, cfg Config) (url.Values, error) {
	values := make(url.Values)

	uq, err := convertQueryToUnified(q)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(uq)
	if err != nil {
		return nil, newError(fmt.Errorf("failed to marshal unified query: %w", err))
	}

	// Only set the parameter if there's actual content
	if string(data) != "{}" {
		values.Set(cfg.ParamNames.Query, string(data))
	}

	return values, nil
}

// convertQueryToUnified converts an r3.Query into the unified JSON structure.
//
// The URL wire schema (unifiedQuery) only carries fields/filters/sorts/pagination by design;
// Cursor, Preloads, GroupBy, Buckets, Aggregates, Having and IncludeTrashed have no wire form
// and do not round-trip through serialization dialects (see r3.Has docs). TODO verify whether
// formatting a Query that carries these should error instead of silently dropping them.
//
//nolint:lostfield // wire schema carries only fields/filters/sorts/pagination by design, see doc above
func convertQueryToUnified(q r3.Query) (*unifiedQuery, error) {
	uq := &unifiedQuery{}

	// Fields
	if len(q.Fields) > 0 {
		fields := make(r3json.JSONFields, len(q.Fields))
		for i, f := range q.Fields {
			fields[i] = r3json.FieldToJSON(f)
		}
		uq.Fields = fields
	}

	// Filters
	if len(q.Filters) > 0 {
		jsonFilters := make(r3json.JSONFilters, 0, len(q.Filters))
		for _, f := range q.Filters {
			jf, err := r3json.FilterToJSON(f)
			if err != nil {
				return nil, newError(fmt.Errorf("failed to convert filter: %w", err))
			}
			jsonFilters = append(jsonFilters, jf)
		}
		uq.Filters = jsonFilters
	}

	// Sorts
	if len(q.Sorts) > 0 {
		jsonSorts := make([]*r3json.JSONSort, 0, len(q.Sorts))
		for _, s := range q.Sorts {
			jsonSorts = append(jsonSorts, r3json.SortToJSON(s))
		}
		uq.Sorts = jsonSorts
	}

	// Pagination
	if q.Pagination != nil {
		uq.Pagination = r3json.PaginationToJSON(q.Pagination)
	}

	return uq, nil
}
