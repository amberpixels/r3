package r3url_test

import (
	"net/url"
	"testing"

	"github.com/expectto/be"
	"github.com/expectto/be/be_reflected"
	"github.com/expectto/be/be_string"
	"github.com/expectto/be/be_url"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
)

func TestParseDjangoFilters(t *testing.T) {
	cfg := r3url.DefaultConfig()
	cfg.Filter.AllowDjangoStyle = true
	cfg.Filter.DjangoFields = []string{"status", "age", "name", "tags"}

	tests := []struct {
		name     string
		values   url.Values
		validate func(t *testing.T, result r3.Filters)
	}{
		{
			name:   "simple eq (bare field)",
			values: url.Values{"status": {"active"}},
			validate: func(t *testing.T, result r3.Filters) {
				be.RequireThat(t, result, be.HaveLength(1))
				be.AssertThat(t, result[0].Field.String(), be.Eq("status"))
				be.AssertThat(t, result[0].Operator, be.Eq(r3.OperatorEq))
				be.AssertThat(t, result[0].Value, be.Eq("active"))
			},
		},
		{
			name:   "operator suffix",
			values: url.Values{"age__gte": {"18"}},
			validate: func(t *testing.T, result r3.Filters) {
				be.RequireThat(t, result, be.HaveLength(1))
				be.AssertThat(t, result[0].Field.String(), be.Eq("age"))
				be.AssertThat(t, result[0].Operator, be.Eq(r3.OperatorGte))
				be.AssertThat(t, result[0].Value, be.Eq("18"))
			},
		},
		{
			name:   "in operator with comma values",
			values: url.Values{"tags__in": {"a,b,c"}},
			validate: func(t *testing.T, result r3.Filters) {
				be.RequireThat(t, result, be.HaveLength(1))
				be.AssertThat(t, result[0].Field.String(), be.Eq("tags"))
				be.AssertThat(t, result[0].Operator, be.Eq(r3.OperatorIn))
				be.AssertThat(t, result[0].Value, be.Eq([]any{"a", "b", "c"}))
			},
		},
		{
			name:   "reserved params are skipped",
			values: url.Values{"status": {"active"}, "page": {"2"}, "fields": {"id,name"}},
			validate: func(t *testing.T, result r3.Filters) {
				be.RequireThat(t, result, be.HaveLength(1))
				be.AssertThat(t, result[0].Field.String(), be.Eq("status"))
			},
		},
		{
			name:   "unknown field skipped (whitelist)",
			values: url.Values{"unknown_field": {"value"}},
			validate: func(t *testing.T, result r3.Filters) {
				be.AssertThat(t, result, be.Empty())
			},
		},
		{
			name:   "unknown operator treated as non-filter",
			values: url.Values{"status__xyz": {"value"}},
			validate: func(t *testing.T, result r3.Filters) {
				be.AssertThat(t, result, be.Empty())
			},
		},
		{
			name:   "like operator",
			values: url.Values{"name__like": {"John%"}},
			validate: func(t *testing.T, result r3.Filters) {
				be.RequireThat(t, result, be.HaveLength(1))
				be.AssertThat(t, result[0].Field.String(), be.Eq("name"))
				be.AssertThat(t, result[0].Operator, be.Eq(r3.OperatorLike))
				be.AssertThat(t, result[0].Value, be.Eq("John%"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3url.ParseDjangoFilters(tt.values, cfg)
			be.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestParseDjangoFilters_Deterministic verifies the filter order is stable across
// runs. Ranging a Go map is randomized, so parsing the same query string must
// still yield filters in a fixed (sorted-by-key) order for filter-hash/caching.
func TestParseDjangoFilters_Deterministic(t *testing.T) {
	cfg := r3url.DefaultConfig()
	cfg.Filter.AllowDjangoStyle = true
	cfg.Filter.DjangoFields = []string{"status", "age", "name", "tags", "country"}

	values := url.Values{
		"status":   {"active"},
		"age__gte": {"18"},
		"name":     {"John"},
		"tags__in": {"a,b"},
		"country":  {"US"},
	}

	first, err := r3url.ParseDjangoFilters(values, cfg)
	be.NoError(t, err)
	be.RequireThat(t, first, be.HaveLength(5))

	fields := make([]string, len(first))
	for i, f := range first {
		fields[i] = f.Field.String()
	}
	// Sorted by key: "age", "country", "name", "status", "tags".
	be.AssertThat(t, fields, be.Eq([]string{"age", "country", "name", "status", "tags"}))

	// Re-parsing many times must produce the identical order.
	for range 50 {
		again, err := r3url.ParseDjangoFilters(values, cfg)
		be.NoError(t, err)
		got := make([]string, len(again))
		for i, f := range again {
			got[i] = f.Field.String()
		}
		be.AssertThat(t, got, be.Eq(fields))
	}
}

func TestParseDjangoFilters_Disabled(t *testing.T) {
	cfg := r3url.DefaultConfig() // AllowDjangoStyle = false by default
	values := url.Values{"status": {"active"}}

	result, err := r3url.ParseDjangoFilters(values, cfg)
	be.NoError(t, err)
	be.AssertThat(t, result, be.Nil())
}

func TestParseDjangoFilters_AllFieldsAllowed(t *testing.T) {
	cfg := r3url.DefaultConfig()
	cfg.Filter.AllowDjangoStyle = true
	// No DjangoFields set — all fields allowed

	values := url.Values{"status": {"active"}, "age__gte": {"18"}}
	result, err := r3url.ParseDjangoFilters(values, cfg)
	be.NoError(t, err)
	be.AssertThat(t, result, be.HaveLength(2))
}

func TestFormatDjangoFilters(t *testing.T) {
	cfg := r3url.DefaultConfig()
	cfg.Filter.AllowDjangoStyle = true

	filters := r3.Filters{
		&r3.FilterSpec{
			Field:    r3.NewFieldSpec("status"),
			Operator: r3.OperatorEq,
			Value:    "active",
		},
		&r3.FilterSpec{
			Field:    r3.NewFieldSpec("age"),
			Operator: r3.OperatorGte,
			Value:    18,
		},
	}

	result := r3url.FormatDjangoFilters(filters, cfg)

	// Assert on the produced url.Values via be_url by wrapping them in a *url.URL.
	u := &url.URL{RawQuery: result.Encode()}
	be.AssertThat(t, u, be_url.URL(
		// plain-value form: bare field for the default "eq" operator
		be_url.HavingSearchParam("status", "active"),
		// matcher-value form: the "__gte" suffix encodes to "age__gte"
		// and the value "18" must read as a numeric string.
		be_url.HavingSearchParam("age__gte", be_reflected.AsNumericString()),
	))
}

func TestFormatDjangoFilters_SkipsGroups(t *testing.T) {
	cfg := r3url.DefaultConfig()
	cfg.Filter.AllowDjangoStyle = true

	filters := r3.Filters{
		&r3.FilterSpec{
			And: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("a"),
					Operator: r3.OperatorEq,
					Value:    "1",
				},
			},
		},
	}

	result := r3url.FormatDjangoFilters(filters, cfg)

	// AND/OR groups are not representable in Django style, so nothing is emitted.
	u := &url.URL{RawQuery: result.Encode()}
	be.AssertThat(t, u, be_url.URL(be_url.HavingRawQuery("")))
}

// TestFormatQuery_DjangoEndToEnd exercises the full FormatQuery -> url.Values path
// and asserts the produced query string via be_url matchers, covering both
// plain-value and matcher-value HavingSearchParam forms plus multi-value params.
func TestFormatQuery_DjangoEndToEnd(t *testing.T) {
	q := r3.Query{
		Fields: r3.Fields{r3.NewFieldSpec("id"), r3.NewFieldSpec("name")},
		Filters: r3.Filters{
			&r3.FilterSpec{
				Field:    r3.NewFieldSpec("status"),
				Operator: r3.OperatorEq,
				Value:    "active",
			},
			&r3.FilterSpec{
				Field:    r3.NewFieldSpec("age"),
				Operator: r3.OperatorGt,
				Value:    18,
			},
			&r3.FilterSpec{
				Field:    r3.NewFieldSpec("tags"),
				Operator: r3.OperatorIn,
				Value:    []any{"a", "b", "c"},
			},
		},
		Sorts: r3.Sorts{
			&r3.SortSpec{Column: r3.NewFieldSpec("name"), Direction: r3.SortDirectionAsc},
		},
		Pagination: r3.NewPaginationSpec(1, 25),
	}

	values, err := r3url.FormatQuery(q, r3url.WithDjangoStyleFilters("status", "age", "tags"))
	be.AssertThat(t, err, be.Succeed())

	u := &url.URL{RawQuery: values.Encode()}
	be.AssertThat(t, u, be_url.URL(
		// Django-style filters: bare field for eq, "__gt" suffix for gt.
		be_url.HavingSearchParam("status", "active"),
		be_url.HavingSearchParam("age__gt", be_reflected.AsNumericString()),
		// "in" values are comma-joined into a single param value.
		be_url.HavingSearchParam("tags__in", be_string.ContainingSubstring("b")),
		// Reserved decomposed params.
		be_url.HavingSearchParam("fields", be_string.ContainingSubstring("name")),
		be_url.HavingSearchParam("sort", "name:asc"),
		be_url.HavingSearchParam("page", "1"),
		be_url.HavingSearchParam("page_size", "25"),
	))

	// HavingMultipleSearchParam reads the []string slice for a key; each Django
	// param here is single-valued, so the slice has exactly one element.
	be.AssertThat(t, u, be_url.HavingMultipleSearchParam("status", be.HaveLength(1)))

	// be.HaveKeyWithValue works directly on url.Values (a map[string][]string).
	be.AssertThat(t, values, be.HaveKeyWithValue("status", []string{"active"}))
}
