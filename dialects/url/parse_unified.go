package r3url

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

// unifiedQuery is the JSON shape of the unified "query" parameter. It mirrors the r3json
// types so ?query={...} shares the JSON dialect's schema.
type unifiedQuery struct {
	Fields     r3json.JSONFields      `json:"fields,omitempty"`
	Filters    r3json.JSONFilters     `json:"filters,omitempty"`
	Sorts      []*r3json.JSONSort     `json:"sorts,omitempty"`
	Pagination *r3json.JSONPagination `json:"pagination,omitempty"`
}

// parseUnified parses the unified ?query={...} parameter into an r3.Query. An
// opted-in ?when= filter is applied on top, so it is not silently dropped when a
// request carries both params.
func parseUnified(values url.Values, cfg Config) (r3.Query, error) {
	q := r3.NewQuery()

	if raw := values.Get(cfg.ParamNames.Query); raw != "" {
		var uq unifiedQuery
		if err := json.Unmarshal([]byte(raw), &uq); err != nil {
			return r3.Query{}, newError(fmt.Errorf("failed to parse unified query JSON: %w", err))
		}
		var err error
		if q, err = convertUnifiedToQuery(uq); err != nil {
			return r3.Query{}, err
		}
	}

	return applyWhenFilter(values, cfg, q)
}

// convertUnifiedToQuery converts the parsed JSON structure into an r3.Query.
func convertUnifiedToQuery(uq unifiedQuery) (r3.Query, error) {
	q := r3.NewQuery()

	// Fields
	if len(uq.Fields) > 0 {
		fields, err := uq.Fields.ToFieldSpecs()
		if err != nil {
			return r3.Query{}, newError(fmt.Errorf("failed to convert fields: %w", err))
		}
		q.Fields = fields
	}

	// Filters
	if len(uq.Filters) > 0 {
		filters, err := r3json.JSONToFilters(uq.Filters)
		if err != nil {
			return r3.Query{}, newError(fmt.Errorf("failed to convert filters: %w", err))
		}
		q.Filters = filters
	}

	// Sorts
	if len(uq.Sorts) > 0 {
		sorts := make(r3.Sorts, 0, len(uq.Sorts))
		for _, js := range uq.Sorts {
			s, err := r3json.JSONToSort(js)
			if err != nil {
				return r3.Query{}, newError(fmt.Errorf("failed to convert sort: %w", err))
			}
			sorts = append(sorts, s)
		}
		q.Sorts = sorts
	}

	// Pagination
	if uq.Pagination != nil {
		p, err := r3json.JSONToPagination(uq.Pagination)
		if err != nil {
			return r3.Query{}, newError(fmt.Errorf("failed to convert pagination: %w", err))
		}
		q.Pagination = p
	}

	return q, nil
}
