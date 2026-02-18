package r3url

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

// unifiedQuery is the JSON structure expected in the unified "query" parameter.
// It mirrors the r3json types so that ?query={...} uses the same schema as the JSON dialect.
type unifiedQuery struct {
	Fields     r3json.JSONFields      `json:"fields,omitempty"`
	Filters    r3json.JSONFilters     `json:"filters,omitempty"`
	Sorts      []*r3json.JSONSort     `json:"sorts,omitempty"`
	Pagination *r3json.JSONPagination `json:"pagination,omitempty"`
}

// parseUnified parses the unified ?query={...} parameter into an r3.Query.
func parseUnified(values url.Values, cfg Config) (r3.Query, error) {
	raw := values.Get(cfg.ParamNames.Query)
	if raw == "" {
		return r3.NewQuery(), nil
	}

	var uq unifiedQuery
	if err := json.Unmarshal([]byte(raw), &uq); err != nil {
		return r3.Query{}, newError(fmt.Errorf("failed to parse unified query JSON: %w", err))
	}

	return convertUnifiedToQuery(uq)
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
