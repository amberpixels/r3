package r3json_test

import (
	"fmt"
	"testing"

	r3json "github.com/amberpixels/r3/dialects/json"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONFiltersToFilters(t *testing.T) {
	tests := []struct {
		name        string
		input       r3json.JSONFilters
		expectError bool
		validate    func(t *testing.T, result r3.Filters)
	}{
		{
			name: "single filter conversion",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField("id"),
					Op:    r3json.OperatorEq,
					Value: 123,
				},
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				spec := result[0].(*r3.FilterSpec)
				assert.Equal(t, "id", spec.Field.String())
				assert.Equal(t, r3.OperatorEq, spec.Operator)
				assert.Equal(t, 123, spec.Value)
			},
		},
		{
			name: "multiple filters conversion",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField("name"),
					Op:    r3json.OperatorLike,
					Value: "john%",
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("age"),
					Op:    r3json.OperatorGte,
					Value: 18,
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("status"),
					Op:    r3json.OperatorIn,
					Value: []string{"active", "pending"},
				},
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 3)

				// First filter - name LIKE 'john%'
				spec1 := result[0].(*r3.FilterSpec)
				assert.Equal(t, "name", spec1.Field.String())
				assert.Equal(t, r3.OperatorLike, spec1.Operator)
				assert.Equal(t, "john%", spec1.Value)

				// Second filter - age >= 18
				spec2 := result[1].(*r3.FilterSpec)
				assert.Equal(t, "age", spec2.Field.String())
				assert.Equal(t, r3.OperatorGte, spec2.Operator)
				assert.Equal(t, 18, spec2.Value)

				// Third filter - status IN ('active', 'pending')
				spec3 := result[2].(*r3.FilterSpec)
				assert.Equal(t, "status", spec3.Field.String())
				assert.Equal(t, r3.OperatorIn, spec3.Operator)
				assert.Equal(t, []string{"active", "pending"}, spec3.Value)
			},
		},
		{
			name: "complex nested filters conversion",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
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
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				spec := result[0].(*r3.FilterSpec)

				// Should have AND conditions
				require.Len(t, spec.And, 2)

				// First AND condition: category = 'electronics'
				firstAnd := spec.And[0].(*r3.FilterSpec)
				assert.Equal(t, "category", firstAnd.Field.String())
				assert.Equal(t, r3.OperatorEq, firstAnd.Operator)
				assert.Equal(t, "electronics", firstAnd.Value)

				// Second AND condition: (price < 100 OR on_sale = true)
				secondAnd := spec.And[1].(*r3.FilterSpec)
				require.Len(t, secondAnd.Or, 2)

				// First OR condition: price < 100
				firstOr := secondAnd.Or[0].(*r3.FilterSpec)
				assert.Equal(t, "price", firstOr.Field.String())
				assert.Equal(t, r3.OperatorLt, firstOr.Operator)
				assert.Equal(t, 100, firstOr.Value)

				// Second OR condition: on_sale = true
				secondOr := secondAnd.Or[1].(*r3.FilterSpec)
				assert.Equal(t, "on_sale", secondOr.Field.String())
				assert.Equal(t, r3.OperatorEq, secondOr.Operator)
				assert.Equal(t, true, secondOr.Value)
			},
		},
		{
			name:  "empty filters conversion",
			input: r3json.JSONFilters{},
			validate: func(t *testing.T, result r3.Filters) {
				assert.Empty(t, result)
			},
		},
		{
			name: "nil value filters conversion",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField("deleted_at"),
					Op:    r3json.OperatorEq,
					Value: nil,
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("archived_at"),
					Op:    r3json.OperatorNe,
					Value: nil,
				},
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 2)

				// First filter: deleted_at IS NULL
				spec1 := result[0].(*r3.FilterSpec)
				assert.Equal(t, "deleted_at", spec1.Field.String())
				assert.Equal(t, r3.OperatorEq, spec1.Operator)
				assert.Nil(t, spec1.Value)

				// Second filter: archived_at IS NOT NULL
				spec2 := result[1].(*r3.FilterSpec)
				assert.Equal(t, "archived_at", spec2.Field.String())
				assert.Equal(t, r3.OperatorNe, spec2.Operator)
				assert.Nil(t, spec2.Value)
			},
		},
		{
			name: "mixed data types conversion",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField("id"),
					Op:    r3json.OperatorEq,
					Value: 42,
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("score"),
					Op:    r3json.OperatorGte,
					Value: 85.5,
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("active"),
					Op:    r3json.OperatorEq,
					Value: true,
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("tags"),
					Op:    r3json.OperatorIn,
					Value: []any{"golang", "testing", "r3"},
				},
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 4)

				// Integer value
				spec1 := result[0].(*r3.FilterSpec)
				assert.Equal(t, "id", spec1.Field.String())
				assert.Equal(t, r3.OperatorEq, spec1.Operator)
				assert.Equal(t, 42, spec1.Value)

				// Float value
				spec2 := result[1].(*r3.FilterSpec)
				assert.Equal(t, "score", spec2.Field.String())
				assert.Equal(t, r3.OperatorGte, spec2.Operator)
				assert.InDelta(t, 85.5, spec2.Value, 0)

				// Boolean value
				spec3 := result[2].(*r3.FilterSpec)
				assert.Equal(t, "active", spec3.Field.String())
				assert.Equal(t, r3.OperatorEq, spec3.Operator)
				assert.Equal(t, true, spec3.Value)

				// Array value
				spec4 := result[3].(*r3.FilterSpec)
				assert.Equal(t, "tags", spec4.Field.String())
				assert.Equal(t, r3.OperatorIn, spec4.Operator)
				assert.Equal(t, []any{"golang", "testing", "r3"}, spec4.Value)
			},
		},
		{
			name: "all supported operators",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{Field: r3json.JSONField("f1"), Op: r3json.OperatorEq, Value: "eq"},
				&r3json.JSONFilter{Field: r3json.JSONField("f2"), Op: r3json.OperatorNe, Value: "ne"},
				&r3json.JSONFilter{Field: r3json.JSONField("f3"), Op: r3json.OperatorGt, Value: 10},
				&r3json.JSONFilter{Field: r3json.JSONField("f4"), Op: r3json.OperatorGte, Value: 10},
				&r3json.JSONFilter{Field: r3json.JSONField("f5"), Op: r3json.OperatorLt, Value: 10},
				&r3json.JSONFilter{Field: r3json.JSONField("f6"), Op: r3json.OperatorLte, Value: 10},
				&r3json.JSONFilter{Field: r3json.JSONField("f7"), Op: r3json.OperatorLike, Value: "like%"},
				&r3json.JSONFilter{Field: r3json.JSONField("f8"), Op: r3json.OperatorNotLike, Value: "notlike%"},
				&r3json.JSONFilter{Field: r3json.JSONField("f9"), Op: r3json.OperatorILike, Value: "ilike%"},
				&r3json.JSONFilter{Field: r3json.JSONField("f10"), Op: r3json.OperatorIn, Value: []string{"a", "b"}},
				&r3json.JSONFilter{Field: r3json.JSONField("f11"), Op: r3json.OperatorNotIn, Value: []string{"c", "d"}},
				&r3json.JSONFilter{Field: r3json.JSONField("f12"), Op: r3json.OperatorBetween, Value: []int{1, 10}},
				&r3json.JSONFilter{Field: r3json.JSONField("f13"), Op: r3json.OperatorBetweenEx, Value: []int{1, 10}},
				&r3json.JSONFilter{
					Field: r3json.JSONField("f14"),
					Op:    r3json.OperatorBetweenExInc,
					Value: []int{1, 10},
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("f15"),
					Op:    r3json.OperatorBetweenIncEx,
					Value: []int{1, 10},
				},
				&r3json.JSONFilter{Field: r3json.JSONField("f16"), Op: r3json.OperatorExists, Value: true},
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 16)

				expectedOperators := []r3.FilterOperatorSpec{
					r3.OperatorEq, r3.OperatorNe, r3.OperatorGt, r3.OperatorGte,
					r3.OperatorLt, r3.OperatorLte, r3.OperatorLike, r3.OperatorNotLike,
					r3.OperatorILike, r3.OperatorIn, r3.OperatorNotIn, r3.OperatorBetween,
					r3.OperatorBetweenEx, r3.OperatorBetweenExInc, r3.OperatorBetweenIncEx,
					r3.OperatorExists,
				}

				for i, expectedOp := range expectedOperators {
					spec := result[i].(*r3.FilterSpec)
					assert.Equal(t, expectedOp, spec.Operator, "Operator mismatch at index %d", i)
					expectedField := fmt.Sprintf("f%d", i+1)
					assert.Equal(t, expectedField, spec.Field.String(), "Field mismatch at index %d", i)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3json.JSONFiltersToFilters(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestJSONFiltersToFilters_Integration(t *testing.T) {
	// Test realistic scenarios that might be used in production
	tests := []struct {
		name     string
		scenario string
		input    r3json.JSONFilters
		validate func(t *testing.T, result r3.Filters)
	}{
		{
			name:     "user search scenario",
			scenario: "Find active users named John with age >= 25",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					And: r3json.JSONFilters{
						&r3json.JSONFilter{
							Field: r3json.JSONField("status"),
							Op:    r3json.OperatorEq,
							Value: "active",
						},
						&r3json.JSONFilter{
							Field: r3json.JSONField("name"),
							Op:    r3json.OperatorLike,
							Value: "John%",
						},
						&r3json.JSONFilter{
							Field: r3json.JSONField("age"),
							Op:    r3json.OperatorGte,
							Value: 25,
						},
					},
				},
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				spec := result[0].(*r3.FilterSpec)
				require.Len(t, spec.And, 3)

				statusFilter := spec.And[0].(*r3.FilterSpec)
				assert.Equal(t, "status", statusFilter.Field.String())
				assert.Equal(t, r3.OperatorEq, statusFilter.Operator)
				assert.Equal(t, "active", statusFilter.Value)

				nameFilter := spec.And[1].(*r3.FilterSpec)
				assert.Equal(t, "name", nameFilter.Field.String())
				assert.Equal(t, r3.OperatorLike, nameFilter.Operator)
				assert.Equal(t, "John%", nameFilter.Value)

				ageFilter := spec.And[2].(*r3.FilterSpec)
				assert.Equal(t, "age", ageFilter.Field.String())
				assert.Equal(t, r3.OperatorGte, ageFilter.Operator)
				assert.Equal(t, 25, ageFilter.Value)
			},
		},
		{
			name:     "product filtering scenario",
			scenario: "Find electronics or books with price between $10-$100 OR on sale",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					And: r3json.JSONFilters{
						&r3json.JSONFilter{
							Or: r3json.JSONFilters{
								&r3json.JSONFilter{
									Field: r3json.JSONField("category"),
									Op:    r3json.OperatorEq,
									Value: "electronics",
								},
								&r3json.JSONFilter{
									Field: r3json.JSONField("category"),
									Op:    r3json.OperatorEq,
									Value: "books",
								},
							},
						},
						&r3json.JSONFilter{
							Or: r3json.JSONFilters{
								&r3json.JSONFilter{
									Field: r3json.JSONField("price"),
									Op:    r3json.OperatorBetween,
									Value: []float64{10.0, 100.0},
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
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				spec := result[0].(*r3.FilterSpec)
				require.Len(t, spec.And, 2)

				// First AND: category in (electronics, books)
				categoryGroup := spec.And[0].(*r3.FilterSpec)
				require.Len(t, categoryGroup.Or, 2)

				// Second AND: (price BETWEEN 10-100 OR on_sale = true)
				priceOrSaleGroup := spec.And[1].(*r3.FilterSpec)
				require.Len(t, priceOrSaleGroup.Or, 2)

				priceFilter := priceOrSaleGroup.Or[0].(*r3.FilterSpec)
				assert.Equal(t, "price", priceFilter.Field.String())
				assert.Equal(t, r3.OperatorBetween, priceFilter.Operator)

				saleFilter := priceOrSaleGroup.Or[1].(*r3.FilterSpec)
				assert.Equal(t, "on_sale", saleFilter.Field.String())
				assert.Equal(t, r3.OperatorEq, saleFilter.Operator)
				assert.Equal(t, true, saleFilter.Value)
			},
		},
		{
			name:     "audit log scenario",
			scenario: "Find logs from last week excluding system user actions",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					And: r3json.JSONFilters{
						&r3json.JSONFilter{
							Field: r3json.JSONField("created_at"),
							Op:    r3json.OperatorGte,
							Value: "2023-01-01T00:00:00Z",
						},
						&r3json.JSONFilter{
							Field: r3json.JSONField("created_at"),
							Op:    r3json.OperatorLte,
							Value: "2023-01-07T23:59:59Z",
						},
						&r3json.JSONFilter{
							Field: r3json.JSONField("user_id"),
							Op:    r3json.OperatorNotIn,
							Value: []int{0, -1}, // system users
						},
						&r3json.JSONFilter{
							Field: r3json.JSONField("deleted_at"),
							Op:    r3json.OperatorEq,
							Value: nil, // not deleted
						},
					},
				},
			},
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				spec := result[0].(*r3.FilterSpec)
				require.Len(t, spec.And, 4)

				// created_at >= start
				startFilter := spec.And[0].(*r3.FilterSpec)
				assert.Equal(t, "created_at", startFilter.Field.String())
				assert.Equal(t, r3.OperatorGte, startFilter.Operator)

				// created_at <= end
				endFilter := spec.And[1].(*r3.FilterSpec)
				assert.Equal(t, "created_at", endFilter.Field.String())
				assert.Equal(t, r3.OperatorLte, endFilter.Operator)

				// user_id NOT IN (0, -1)
				userFilter := spec.And[2].(*r3.FilterSpec)
				assert.Equal(t, "user_id", userFilter.Field.String())
				assert.Equal(t, r3.OperatorNotIn, userFilter.Operator)
				assert.Equal(t, []int{0, -1}, userFilter.Value)

				// deleted_at IS NULL
				deletedFilter := spec.And[3].(*r3.FilterSpec)
				assert.Equal(t, "deleted_at", deletedFilter.Field.String())
				assert.Equal(t, r3.OperatorEq, deletedFilter.Operator)
				assert.Nil(t, deletedFilter.Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3json.JSONFiltersToFilters(tt.input)
			require.NoError(t, err, "Scenario: %s", tt.scenario)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestJSONFiltersToFilters_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       r3json.JSONFilters
		expectError bool
		description string
	}{
		{
			name:        "nil slice",
			input:       nil,
			expectError: false,
			description: "Should handle nil JSONFilters gracefully",
		},
		{
			name: "filters with empty fields",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField(""),
					Op:    r3json.OperatorEq,
					Value: "test",
				},
			},
			expectError: false,
			description: "Should handle empty field names",
		},
		{
			name: "deeply nested filters",
			input: r3json.JSONFilters{
				&r3json.JSONFilter{
					And: r3json.JSONFilters{
						&r3json.JSONFilter{
							Or: r3json.JSONFilters{
								&r3json.JSONFilter{
									And: r3json.JSONFilters{
										&r3json.JSONFilter{
											Field: r3json.JSONField("level1"),
											Op:    r3json.OperatorEq,
											Value: "deep",
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
			description: "Should handle deeply nested filter structures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3json.JSONFiltersToFilters(tt.input)

			if tt.expectError {
				require.Error(t, err, tt.description)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err, tt.description)
				// Just verify it doesn't panic and returns something reasonable
				assert.NotNil(t, result)
			}
		})
	}
}

func BenchmarkJSONFiltersToFilters(b *testing.B) {
	// Create a reasonably complex filter set for benchmarking
	filters := r3json.JSONFilters{
		&r3json.JSONFilter{
			And: r3json.JSONFilters{
				&r3json.JSONFilter{
					Field: r3json.JSONField("status"),
					Op:    r3json.OperatorIn,
					Value: []string{"active", "pending", "approved"},
				},
				&r3json.JSONFilter{
					Or: r3json.JSONFilters{
						&r3json.JSONFilter{
							Field: r3json.JSONField("priority"),
							Op:    r3json.OperatorGte,
							Value: 5,
						},
						&r3json.JSONFilter{
							Field: r3json.JSONField("urgent"),
							Op:    r3json.OperatorEq,
							Value: true,
						},
					},
				},
				&r3json.JSONFilter{
					Field: r3json.JSONField("created_at"),
					Op:    r3json.OperatorGte,
					Value: "2023-01-01T00:00:00Z",
				},
			},
		},
		&r3json.JSONFilter{
			Field: r3json.JSONField("deleted_at"),
			Op:    r3json.OperatorEq,
			Value: nil,
		},
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := r3json.JSONFiltersToFilters(filters)
		if err != nil {
			b.Fatal(err)
		}
	}
}
