package r3url_test

import (
	"net/url"
	"testing"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				require.Len(t, result, 1)
				assert.Equal(t, "status", result[0].Field.String())
				assert.Equal(t, r3.OperatorEq, result[0].Operator)
				assert.Equal(t, "active", result[0].Value)
			},
		},
		{
			name:   "operator suffix",
			values: url.Values{"age__gte": {"18"}},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				assert.Equal(t, "age", result[0].Field.String())
				assert.Equal(t, r3.OperatorGte, result[0].Operator)
				assert.Equal(t, "18", result[0].Value)
			},
		},
		{
			name:   "in operator with comma values",
			values: url.Values{"tags__in": {"a,b,c"}},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				assert.Equal(t, "tags", result[0].Field.String())
				assert.Equal(t, r3.OperatorIn, result[0].Operator)
				assert.Equal(t, []any{"a", "b", "c"}, result[0].Value)
			},
		},
		{
			name:   "reserved params are skipped",
			values: url.Values{"status": {"active"}, "page": {"2"}, "fields": {"id,name"}},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				assert.Equal(t, "status", result[0].Field.String())
			},
		},
		{
			name:   "unknown field skipped (whitelist)",
			values: url.Values{"unknown_field": {"value"}},
			validate: func(t *testing.T, result r3.Filters) {
				assert.Empty(t, result)
			},
		},
		{
			name:   "unknown operator treated as non-filter",
			values: url.Values{"status__xyz": {"value"}},
			validate: func(t *testing.T, result r3.Filters) {
				assert.Empty(t, result)
			},
		},
		{
			name:   "like operator",
			values: url.Values{"name__like": {"John%"}},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				assert.Equal(t, "name", result[0].Field.String())
				assert.Equal(t, r3.OperatorLike, result[0].Operator)
				assert.Equal(t, "John%", result[0].Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3url.ParseDjangoFilters(tt.values, cfg)
			require.NoError(t, err)
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
	require.NoError(t, err)
	require.Len(t, first, 5)

	fields := make([]string, len(first))
	for i, f := range first {
		fields[i] = f.Field.String()
	}
	// Sorted by key: "age", "country", "name", "status", "tags".
	assert.Equal(t, []string{"age", "country", "name", "status", "tags"}, fields)

	// Re-parsing many times must produce the identical order.
	for range 50 {
		again, err := r3url.ParseDjangoFilters(values, cfg)
		require.NoError(t, err)
		got := make([]string, len(again))
		for i, f := range again {
			got[i] = f.Field.String()
		}
		assert.Equal(t, fields, got)
	}
}

func TestParseDjangoFilters_Disabled(t *testing.T) {
	cfg := r3url.DefaultConfig() // AllowDjangoStyle = false by default
	values := url.Values{"status": {"active"}}

	result, err := r3url.ParseDjangoFilters(values, cfg)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestParseDjangoFilters_AllFieldsAllowed(t *testing.T) {
	cfg := r3url.DefaultConfig()
	cfg.Filter.AllowDjangoStyle = true
	// No DjangoFields set — all fields allowed

	values := url.Values{"status": {"active"}, "age__gte": {"18"}}
	result, err := r3url.ParseDjangoFilters(values, cfg)
	require.NoError(t, err)
	assert.Len(t, result, 2)
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
	assert.Equal(t, "active", result.Get("status"))
	assert.Equal(t, "18", result.Get("age__gte"))
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
	assert.Empty(t, result)
}
