package r3json_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

func TestJSONFilter_String(t *testing.T) {
	tests := []struct {
		name     string
		filter   *r3json.JSONFilter
		expected string
	}{
		{
			name: "simple filter",
			filter: &r3json.JSONFilter{
				Field: r3json.JSONField("id"),
				Op:    r3json.OperatorEq,
				Value: 123,
			},
			expected: `{"f":"id","op":"eq","v":123}`,
		},
		{
			name: "filter with string value",
			filter: &r3json.JSONFilter{
				Field: r3json.JSONField("name"),
				Op:    r3json.OperatorLike,
				Value: "john%",
			},
			expected: `{"f":"name","op":"like","v":"john%"}`,
		},
		{
			name: "filter with nil value",
			filter: &r3json.JSONFilter{
				Field: r3json.JSONField("deleted_at"),
				Op:    r3json.OperatorEq,
				Value: nil,
			},
			expected: `{"f":"deleted_at","op":"eq","v":null}`,
		},
		{
			name: "filter with AND group",
			filter: &r3json.JSONFilter{
				And: r3json.JSONFilters{
					&r3json.JSONFilter{
						Field: r3json.JSONField("id"),
						Op:    r3json.OperatorGt,
						Value: 10,
					},
					&r3json.JSONFilter{
						Field: r3json.JSONField("status"),
						Op:    r3json.OperatorEq,
						Value: "active",
					},
				},
			},
			expected: `{"and":[{"f":"id","op":"gt","v":10},{"f":"status","op":"eq","v":"active"}]}`,
		},
		{
			name: "filter with OR group",
			filter: &r3json.JSONFilter{
				Or: r3json.JSONFilters{
					&r3json.JSONFilter{
						Field: r3json.JSONField("type"),
						Op:    r3json.OperatorEq,
						Value: "user",
					},
					&r3json.JSONFilter{
						Field: r3json.JSONField("type"),
						Op:    r3json.OperatorEq,
						Value: "admin",
					},
				},
			},
			expected: `{"or":[{"f":"type","op":"eq","v":"user"},{"f":"type","op":"eq","v":"admin"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.String()
			assert.JSONEq(t, tt.expected, result)
		})
	}
}

func TestJSONFilters_String(t *testing.T) {
	tests := []struct {
		name     string
		filters  r3json.JSONFilters
		expected string
	}{
		{
			name: "single filter",
			filters: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField("id"),
					Op:    r3json.OperatorEq,
					Value: 123,
				},
			},
			expected: `[{"f":"id","op":"eq","v":123}]`,
		},
		{
			name: "multiple filters",
			filters: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField("id"),
					Op:    r3json.OperatorGt,
					Value: 10,
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("status"),
					Op:    r3json.OperatorEq,
					Value: "active",
				},
			},
			expected: `[{"f":"id","op":"gt","v":10},{"f":"status","op":"eq","v":"active"}]`,
		},
		{
			name:     "empty filters",
			filters:  r3json.JSONFilters{},
			expected: `[]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filters.String()
			assert.JSONEq(t, tt.expected, result)
		})
	}
}

func TestJSONFilter_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(t *testing.T, filter *r3json.JSONFilter)
	}{
		{
			name:  "simple filter",
			input: `{"f":"id","op":"eq","v":123}`,
			validate: func(t *testing.T, filter *r3json.JSONFilter) {
				assert.Equal(t, r3json.JSONField("id"), filter.Field)
				assert.Equal(t, r3json.OperatorEq, filter.Op)
				assert.InDelta(t, float64(123), filter.Value, 0.0001) // JSON numbers become float64
			},
		},
		{
			name:  "filter with string value",
			input: `{"f":"name","op":"like","v":"john%"}`,
			validate: func(t *testing.T, filter *r3json.JSONFilter) {
				assert.Equal(t, r3json.JSONField("name"), filter.Field)
				assert.Equal(t, r3json.OperatorLike, filter.Op)
				assert.Equal(t, "john%", filter.Value)
			},
		},
		{
			name:  "filter with null value",
			input: `{"f":"deleted_at","op":"eq","v":null}`,
			validate: func(t *testing.T, filter *r3json.JSONFilter) {
				assert.Equal(t, r3json.JSONField("deleted_at"), filter.Field)
				assert.Equal(t, r3json.OperatorEq, filter.Op)
				assert.Nil(t, filter.Value)
			},
		},
		{
			name:  "filter with AND group",
			input: `{"and":[{"f":"id","op":"gt","v":10},{"f":"status","op":"eq","v":"active"}]}`,
			validate: func(t *testing.T, filter *r3json.JSONFilter) {
				require.Len(t, filter.And, 2)
				assert.Equal(t, r3json.JSONField("id"), filter.And[0].Field)
				assert.Equal(t, r3json.OperatorGt, filter.And[0].Op)
				assert.InDelta(t, float64(10), filter.And[0].Value, 0.00001)
				assert.Equal(t, r3json.JSONField("status"), filter.And[1].Field)
				assert.Equal(t, r3json.OperatorEq, filter.And[1].Op)
				assert.Equal(t, "active", filter.And[1].Value)
			},
		},
		{
			name:  "filter with OR group",
			input: `{"or":[{"f":"type","op":"eq","v":"user"},{"f":"type","op":"eq","v":"admin"}]}`,
			validate: func(t *testing.T, filter *r3json.JSONFilter) {
				require.Len(t, filter.Or, 2)
				assert.Equal(t, r3json.JSONField("type"), filter.Or[0].Field)
				assert.Equal(t, r3json.OperatorEq, filter.Or[0].Op)
				assert.Equal(t, "user", filter.Or[0].Value)
				assert.Equal(t, r3json.JSONField("type"), filter.Or[1].Field)
				assert.Equal(t, r3json.OperatorEq, filter.Or[1].Op)
				assert.Equal(t, "admin", filter.Or[1].Value)
			},
		},
		{
			name:        "invalid JSON",
			input:       `{"f":"id","op":"invalid"`,
			expectError: true,
		},
		{
			name:        "invalid operator",
			input:       `{"f":"id","op":"invalid_op","v":123}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filter r3json.JSONFilter
			err := json.Unmarshal([]byte(tt.input), &filter)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, &filter)
				}
			}
		})
	}
}

func TestJSONFilter_ToFilterSpec(t *testing.T) {
	tests := []struct {
		name        string
		filter      *r3json.JSONFilter
		expectError bool
		validate    func(t *testing.T, spec *r3.FilterSpec)
	}{
		{
			name: "simple filter",
			filter: &r3json.JSONFilter{
				Field: r3json.JSONField("id"),
				Op:    r3json.OperatorEq,
				Value: 123,
			},
			validate: func(t *testing.T, spec *r3.FilterSpec) {
				assert.Equal(t, "id", spec.Field.String())
				assert.Equal(t, r3.OperatorEq, spec.Operator)
				assert.Equal(t, 123, spec.Value)
				assert.Empty(t, spec.And)
				assert.Empty(t, spec.Or)
			},
		},
		{
			name: "filter with string value",
			filter: &r3json.JSONFilter{
				Field: r3json.JSONField("name"),
				Op:    r3json.OperatorLike,
				Value: "john%",
			},
			validate: func(t *testing.T, spec *r3.FilterSpec) {
				assert.Equal(t, "name", spec.Field.String())
				assert.Equal(t, r3.OperatorLike, spec.Operator)
				assert.Equal(t, "john%", spec.Value)
			},
		},
		{
			name: "filter with nil value",
			filter: &r3json.JSONFilter{
				Field: r3json.JSONField("deleted_at"),
				Op:    r3json.OperatorEq,
				Value: nil,
			},
			validate: func(t *testing.T, spec *r3.FilterSpec) {
				assert.Equal(t, "deleted_at", spec.Field.String())
				assert.Equal(t, r3.OperatorEq, spec.Operator)
				assert.Nil(t, spec.Value)
			},
		},
		{
			name: "filter with AND group",
			filter: &r3json.JSONFilter{
				And: r3json.JSONFilters{
					&r3json.JSONFilter{
						Field: r3json.JSONField("id"),
						Op:    r3json.OperatorGt,
						Value: 10,
					},
					&r3json.JSONFilter{
						Field: r3json.JSONField("status"),
						Op:    r3json.OperatorEq,
						Value: "active",
					},
				},
			},
			validate: func(t *testing.T, spec *r3.FilterSpec) {
				require.Len(t, spec.And, 2)
				assert.Equal(t, "id", spec.And[0].Field.String())
				assert.Equal(t, r3.OperatorGt, spec.And[0].Operator)
				assert.Equal(t, 10, spec.And[0].Value)
				assert.Equal(t, "status", spec.And[1].Field.String())
				assert.Equal(t, r3.OperatorEq, spec.And[1].Operator)
				assert.Equal(t, "active", spec.And[1].Value)
			},
		},
		{
			name: "filter with OR group",
			filter: &r3json.JSONFilter{
				Or: r3json.JSONFilters{
					&r3json.JSONFilter{
						Field: r3json.JSONField("type"),
						Op:    r3json.OperatorEq,
						Value: "user",
					},
					&r3json.JSONFilter{
						Field: r3json.JSONField("type"),
						Op:    r3json.OperatorEq,
						Value: "admin",
					},
				},
			},
			validate: func(t *testing.T, spec *r3.FilterSpec) {
				require.Len(t, spec.Or, 2)
				assert.Equal(t, "type", spec.Or[0].Field.String())
				assert.Equal(t, r3.OperatorEq, spec.Or[0].Operator)
				assert.Equal(t, "user", spec.Or[0].Value)
				assert.Equal(t, "type", spec.Or[1].Field.String())
				assert.Equal(t, r3.OperatorEq, spec.Or[1].Operator)
				assert.Equal(t, "admin", spec.Or[1].Value)
			},
		},
		{
			name: "complex nested filter",
			filter: &r3json.JSONFilter{
				And: r3json.JSONFilters{
					&r3json.JSONFilter{
						Field: r3json.JSONField("category"),
						Op:    r3json.OperatorEq,
						Value: "electronics",
					},
					&r3json.JSONFilter{
						Or: r3json.JSONFilters{
							&r3json.JSONFilter{
								Field: r3json.JSONField("price"),
								Op:    r3json.OperatorLt,
								Value: 100,
							},
							&r3json.JSONFilter{
								Field: r3json.JSONField("on_sale"),
								Op:    r3json.OperatorEq,
								Value: true,
							},
						},
					},
				},
			},
			validate: func(t *testing.T, spec *r3.FilterSpec) {
				require.Len(t, spec.And, 2)
				// First AND condition
				assert.Equal(t, "category", spec.And[0].Field.String())
				assert.Equal(t, r3.OperatorEq, spec.And[0].Operator)
				assert.Equal(t, "electronics", spec.And[0].Value)

				// Second AND condition (OR group)
				secondCondition := spec.And[1]
				require.Len(t, secondCondition.Or, 2)
				assert.Equal(t, "price", secondCondition.Or[0].Field.String())
				assert.Equal(t, r3.OperatorLt, secondCondition.Or[0].Operator)
				assert.Equal(t, "on_sale", secondCondition.Or[1].Field.String())
				assert.Equal(t, r3.OperatorEq, secondCondition.Or[1].Operator)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.filter.ToFilterSpec()

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// func TestJSONFilters_toFilters(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		filters     r3json.JSONFilters
// 		expectError bool
// 		validate    func(t *testing.T, filters r3.Filters)
// 	}{
// 		{
// 			name: "single filter",
// 			filters: r3json.JSONFilters{
// 				&r3json.JSONFilter{
// 					Field: r3json.JSONField("id"),
// 					Op:    r3json.OperatorEq,
// 					Value: 123,
// 				},
// 			},
// 			validate: func(t *testing.T, filters r3.Filters) {
// 				require.Len(t, filters, 1)
// 				spec := filters[0].(*r3.FilterSpec)
// 				assert.Equal(t, "id", spec.Field.String())
// 				assert.Equal(t, r3.OperatorEq, spec.Operator)
// 				assert.Equal(t, 123, spec.Value)
// 			},
// 		},
// 		{
// 			name: "multiple filters",
// 			filters: r3json.JSONFilters{
// 				&r3json.JSONFilter{
// 					Field: r3json.JSONField("id"),
// 					Op:    r3json.OperatorGt,
// 					Value: 10,
// 				},
// 				&r3json.JSONFilter{
// 					Field: r3json.JSONField("status"),
// 					Op:    r3json.OperatorEq,
// 					Value: "active",
// 				},
// 				&r3json.JSONFilter{
// 					Field: r3json.JSONField("name"),
// 					Op:    r3json.OperatorLike,
// 					Value: "john%",
// 				},
// 			},
// 			validate: func(t *testing.T, filters r3.Filters) {
// 				require.Len(t, filters, 3)

// 				// First filter
// 				spec1 := filters[0].(*r3.FilterSpec)
// 				assert.Equal(t, "id", spec1.Field.String())
// 				assert.Equal(t, r3.OperatorGt, spec1.Operator)
// 				assert.Equal(t, 10, spec1.Value)

// 				// Second filter
// 				spec2 := filters[1].(*r3.FilterSpec)
// 				assert.Equal(t, "status", spec2.Field.String())
// 				assert.Equal(t, r3.OperatorEq, spec2.Operator)
// 				assert.Equal(t, "active", spec2.Value)

// 				// Third filter
// 				spec3 := filters[2].(*r3.FilterSpec)
// 				assert.Equal(t, "name", spec3.Field.String())
// 				assert.Equal(t, r3.OperatorLike, spec3.Operator)
// 				assert.Equal(t, "john%", spec3.Value)
// 			},
// 		},
// 		{
// 			name:    "empty filters",
// 			filters: r3json.JSONFilters{},
// 			validate: func(t *testing.T, filters r3.Filters) {
// 				assert.Empty(t, filters)
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := tt.filters.toFilters()

// 			if tt.expectError {
// 				require.Error(t, err)
// 				assert.Nil(t, result)
// 			} else {
// 				require.NoError(t, err)
// 				if tt.validate != nil {
// 					tt.validate(t, result)
// 				}
// 			}
// 		})
// 	}
// }

func TestJSONFilter_JSONRoundTrip(t *testing.T) {
	originalFilter := &r3json.JSONFilter{
		Field: r3json.JSONField("user.profile.name"),
		Op:    r3json.OperatorLike,
		Value: "john%",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(originalFilter)
	require.NoError(t, err)

	// Unmarshal from JSON
	var unmarshaledFilter r3json.JSONFilter
	err = json.Unmarshal(jsonData, &unmarshaledFilter)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, originalFilter.Field, unmarshaledFilter.Field)
	assert.Equal(t, originalFilter.Op, unmarshaledFilter.Op)
	assert.Equal(t, originalFilter.Value, unmarshaledFilter.Value)
}

func TestJSONFilter_ComplexJSONRoundTrip(t *testing.T) {
	originalFilter := &r3json.JSONFilter{
		And: r3json.JSONFilters{
			&r3json.JSONFilter{
				Field: r3json.JSONField("category"),
				Op:    r3json.OperatorEq,
				Value: "electronics",
			},
			&r3json.JSONFilter{
				Or: r3json.JSONFilters{
					&r3json.JSONFilter{
						Field: r3json.JSONField("price"),
						Op:    r3json.OperatorLt,
						Value: 100,
					},
					&r3json.JSONFilter{
						Field: r3json.JSONField("on_sale"),
						Op:    r3json.OperatorEq,
						Value: true,
					},
				},
			},
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(originalFilter)
	require.NoError(t, err)

	// Unmarshal from JSON
	var unmarshaledFilter r3json.JSONFilter
	err = json.Unmarshal(jsonData, &unmarshaledFilter)
	require.NoError(t, err)

	// Compare structure
	require.Len(t, unmarshaledFilter.And, 2)
	assert.Equal(t, r3json.JSONField("category"), unmarshaledFilter.And[0].Field)
	assert.Equal(t, r3json.OperatorEq, unmarshaledFilter.And[0].Op)
	assert.Equal(t, "electronics", unmarshaledFilter.And[0].Value)

	require.Len(t, unmarshaledFilter.And[1].Or, 2)
	assert.Equal(t, r3json.JSONField("price"), unmarshaledFilter.And[1].Or[0].Field)
	assert.Equal(t, r3json.OperatorLt, unmarshaledFilter.And[1].Or[0].Op)
	assert.Equal(t, r3json.JSONField("on_sale"), unmarshaledFilter.And[1].Or[1].Field)
	assert.Equal(t, r3json.OperatorEq, unmarshaledFilter.And[1].Or[1].Op)
}

func TestJSONFilter_OperatorVariations(t *testing.T) {
	// Test various operators to ensure comprehensive coverage
	operators := []struct {
		jsonOp r3json.JSONFilterOperator
		r3Op   r3.FilterOperatorSpec
	}{
		{r3json.OperatorEq, r3.OperatorEq},
		{r3json.OperatorNe, r3.OperatorNe},
		{r3json.OperatorGt, r3.OperatorGt},
		{r3json.OperatorGte, r3.OperatorGte},
		{r3json.OperatorLt, r3.OperatorLt},
		{r3json.OperatorLte, r3.OperatorLte},
		{r3json.OperatorLike, r3.OperatorLike},
		{r3json.OperatorNotLike, r3.OperatorNotLike},
		{r3json.OperatorILike, r3.OperatorILike},
		{r3json.OperatorIn, r3.OperatorIn},
		{r3json.OperatorNotIn, r3.OperatorNotIn},
		{r3json.OperatorBetween, r3.OperatorBetween},
		{r3json.OperatorBetweenEx, r3.OperatorBetweenEx},
		{r3json.OperatorBetweenExInc, r3.OperatorBetweenExInc},
		{r3json.OperatorBetweenIncEx, r3.OperatorBetweenIncEx},
		{r3json.OperatorExists, r3.OperatorExists},
	}

	for _, op := range operators {
		t.Run(op.jsonOp.String(), func(t *testing.T) {
			filter := &r3json.JSONFilter{
				Field: r3json.JSONField("test_field"),
				Op:    op.jsonOp,
				Value: "test_value",
			}

			spec, err := filter.ToFilterSpec()
			require.NoError(t, err)
			assert.Equal(t, op.r3Op, spec.Operator)
		})
	}
}

func TestJSONFilter_ValueTypes(t *testing.T) {
	// Test different value types
	tests := []struct {
		name  string
		value any
	}{
		{"string", "test"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
		{"array", []string{"a", "b", "c"}},
		{"map", map[string]any{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &r3json.JSONFilter{
				Field: r3json.JSONField("test_field"),
				Op:    r3json.OperatorEq,
				Value: tt.value,
			}

			spec, err := filter.ToFilterSpec()
			require.NoError(t, err)
			assert.Equal(t, tt.value, spec.Value)
		})
	}
}
