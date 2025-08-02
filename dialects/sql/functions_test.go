package r3sql_test

import (
	"testing"

	r3sql "github.com/amberpixels/r3/dialects/sql"

	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiltersToSQLClauses(t *testing.T) {
	tests := []struct {
		name        string
		input       r3.Filters
		expectError bool
		validate    func(t *testing.T, result r3sql.SQLClauses)
	}{
		{
			name:  "empty filters",
			input: r3.Filters{},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				assert.Empty(t, result)
			},
		},
		{
			name:  "nil filters",
			input: nil,
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				assert.Empty(t, result)
			},
		},
		{
			name: "single simple filter",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("id"),
					Operator: r3.OperatorEq,
					Value:    123,
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 1)
				assert.Equal(t, "id = ?", result[0].Clause)
				require.Len(t, result[0].Args, 1)
				assert.Equal(t, 123, result[0].Args[0])
				assert.Empty(t, result[0].Joins)
			},
		},
		{
			name: "multiple simple filters",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("id"),
					Operator: r3.OperatorGt,
					Value:    10,
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("name"),
					Operator: r3.OperatorLike,
					Value:    "John%",
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("status"),
					Operator: r3.OperatorIn,
					Value:    []string{"active", "pending"},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 3)

				// First filter: id > 10
				assert.Equal(t, "id > ?", result[0].Clause)
				require.Len(t, result[0].Args, 1)
				assert.Equal(t, 10, result[0].Args[0])

				// Second filter: name LIKE 'John%'
				assert.Equal(t, "name LIKE ?", result[1].Clause)
				require.Len(t, result[1].Args, 1)
				assert.Equal(t, "John%", result[1].Args[0])

				// Third filter: status IN ('active', 'pending')
				assert.Equal(t, "status IN ?", result[2].Clause)
				require.Len(t, result[2].Args, 1)
				assert.Equal(t, []string{"active", "pending"}, result[2].Args[0])
			},
		},
		{
			name: "filters with joins",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("user.name"),
					Operator: r3.OperatorEq,
					Value:    "John",
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("profile.age"),
					Operator: r3.OperatorGte,
					Value:    18,
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 2)

				// First filter with user join
				assert.Equal(t, "user.name = ?", result[0].Clause)
				require.Len(t, result[0].Args, 1)
				assert.Equal(t, "John", result[0].Args[0])
				require.Len(t, result[0].Joins, 1)
				assert.Equal(t, r3sql.SQLColumn("user"), result[0].Joins[0])

				// Second filter with profile join
				assert.Equal(t, "profile.age >= ?", result[1].Clause)
				require.Len(t, result[1].Args, 1)
				assert.Equal(t, 18, result[1].Args[0])
				require.Len(t, result[1].Joins, 1)
				assert.Equal(t, r3sql.SQLColumn("profile"), result[1].Joins[0])
			},
		},
		{
			name: "complex nested filters",
			input: r3.Filters{
				&r3.FilterSpec{
					And: r3.Filters{
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("category"),
							Operator: r3.OperatorEq,
							Value:    "electronics",
						},
						&r3.FilterSpec{
							Or: r3.Filters{
								&r3.FilterSpec{
									Field:    r3.NewFieldSpec("price"),
									Operator: r3.OperatorLt,
									Value:    100,
								},
								&r3.FilterSpec{
									Field:    r3.NewFieldSpec("on_sale"),
									Operator: r3.OperatorEq,
									Value:    true,
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 1)
				assert.Equal(t, "(category = ? AND (price < ? OR on_sale = ?))", result[0].Clause)
				require.Len(t, result[0].Args, 3)
				assert.Equal(t, "electronics", result[0].Args[0])
				assert.Equal(t, 100, result[0].Args[1])
				assert.Equal(t, true, result[0].Args[2])
			},
		},
		{
			name: "filters with NULL values",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("deleted_at"),
					Operator: r3.OperatorEq,
					Value:    nil,
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("archived_at"),
					Operator: r3.OperatorNe,
					Value:    nil,
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 2)

				// First filter: deleted_at IS NULL
				assert.Equal(t, "deleted_at IS NULL", result[0].Clause)
				assert.Empty(t, result[0].Args)

				// Second filter: archived_at IS NOT NULL
				assert.Equal(t, "archived_at IS NOT NULL", result[1].Clause)
				assert.Empty(t, result[1].Args)
			},
		},
		{
			name: "filters with all supported operators",
			input: r3.Filters{
				&r3.FilterSpec{Field: r3.NewFieldSpec("f1"), Operator: r3.OperatorEq, Value: "eq"},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f2"), Operator: r3.OperatorNe, Value: "ne"},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f3"), Operator: r3.OperatorGt, Value: 10},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f4"), Operator: r3.OperatorGte, Value: 10},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f5"), Operator: r3.OperatorLt, Value: 10},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f6"), Operator: r3.OperatorLte, Value: 10},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f7"), Operator: r3.OperatorLike, Value: "like%"},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f8"), Operator: r3.OperatorNotLike, Value: "notlike%"},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f9"), Operator: r3.OperatorILike, Value: "ilike%"},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f10"), Operator: r3.OperatorIn, Value: []string{"a", "b"}},
				&r3.FilterSpec{Field: r3.NewFieldSpec("f11"), Operator: r3.OperatorNotIn, Value: []string{"c", "d"}},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 11)

				expectedClauses := []string{
					"f1 = ?", "f2 != ?", "f3 > ?", "f4 >= ?", "f5 < ?", "f6 <= ?",
					"f7 LIKE ?", "f8 NOT LIKE ?", "f9 ILIKE ?", "f10 IN ?", "f11 NOT IN ?",
				}

				for i, expectedClause := range expectedClauses {
					assert.Equal(t, expectedClause, result[i].Clause, "Clause mismatch at index %d", i)
					assert.Len(t, result[i].Args, 1, "Should have exactly one arg at index %d", i)
				}
			},
		},
		{
			name: "mixed data types",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("id"),
					Operator: r3.OperatorEq,
					Value:    42,
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("score"),
					Operator: r3.OperatorGte,
					Value:    85.5,
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("active"),
					Operator: r3.OperatorEq,
					Value:    true,
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("tags"),
					Operator: r3.OperatorIn,
					Value:    []any{"golang", "testing", "sql"},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 4)

				// Integer
				assert.Equal(t, "id = ?", result[0].Clause)
				assert.Equal(t, 42, result[0].Args[0])

				// Float
				assert.Equal(t, "score >= ?", result[1].Clause)
				assert.InDelta(t, 85.5, result[1].Args[0], 0.0001)

				// Boolean
				assert.Equal(t, "active = ?", result[2].Clause)
				assert.Equal(t, true, result[2].Args[0])

				// Interface slice
				assert.Equal(t, "tags IN ?", result[3].Clause)
				assert.Equal(t, []any{"golang", "testing", "sql"}, result[3].Args[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.FiltersToSQLClauses(tt.input)

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

func TestFiltersToSQLClauses_Integration(t *testing.T) {
	// Test realistic scenarios that might be used in production
	tests := []struct {
		name     string
		scenario string
		input    r3.Filters
		validate func(t *testing.T, result r3sql.SQLClauses)
	}{
		{
			name:     "user authentication query",
			scenario: "Find active user with specific email and non-null last login",
			input: r3.Filters{
				&r3.FilterSpec{
					And: r3.Filters{
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("email"),
							Operator: r3.OperatorEq,
							Value:    "user@example.com",
						},
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("status"),
							Operator: r3.OperatorEq,
							Value:    "active",
						},
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("last_login"),
							Operator: r3.OperatorNe,
							Value:    nil,
						},
					},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 1)
				assert.Equal(t, "(email = ? AND status = ? AND last_login IS NOT NULL)", result[0].Clause)
				require.Len(t, result[0].Args, 2)
				assert.Equal(t, "user@example.com", result[0].Args[0])
				assert.Equal(t, "active", result[0].Args[1])
				assert.Empty(t, result[0].Joins)
			},
		},
		{
			name:     "product search with joins",
			scenario: "Find products in specific categories with price or discount constraints",
			input: r3.Filters{
				&r3.FilterSpec{
					And: r3.Filters{
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("category.name"),
							Operator: r3.OperatorIn,
							Value:    []string{"electronics", "computers"},
						},
						&r3.FilterSpec{
							Or: r3.Filters{
								&r3.FilterSpec{
									Field:    r3.NewFieldSpec("price"),
									Operator: r3.OperatorLt,
									Value:    1000,
								},
								&r3.FilterSpec{
									Field:    r3.NewFieldSpec("discount.active"),
									Operator: r3.OperatorEq,
									Value:    true,
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 1)
				assert.Equal(t, "(category.name IN ? AND (price < ? OR discount.active = ?))", result[0].Clause)
				require.Len(t, result[0].Args, 3)
				assert.Equal(t, []string{"electronics", "computers"}, result[0].Args[0])
				assert.Equal(t, 1000, result[0].Args[1])
				assert.Equal(t, true, result[0].Args[2])

				// Check joins
				expectedJoins := []r3sql.SQLColumn{r3sql.SQLColumn("category"), r3sql.SQLColumn("discount")}
				assert.ElementsMatch(t, expectedJoins, result[0].Joins)
			},
		},
		{
			name:     "audit log filtering",
			scenario: "Complex audit log query with time ranges and user exclusions",
			input: r3.Filters{
				&r3.FilterSpec{
					And: r3.Filters{
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("created_at"),
							Operator: r3.OperatorGte,
							Value:    "2023-01-01T00:00:00Z",
						},
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("created_at"),
							Operator: r3.OperatorLte,
							Value:    "2023-12-31T23:59:59Z",
						},
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("user.role"),
							Operator: r3.OperatorNotIn,
							Value:    []string{"system", "bot"},
						},
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("deleted_at"),
							Operator: r3.OperatorEq,
							Value:    nil,
						},
					},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 1)
				assert.Equal(
					t,
					"(created_at >= ? AND created_at <= ? AND user.role NOT IN ? AND deleted_at IS NULL)",
					result[0].Clause,
				)
				require.Len(t, result[0].Args, 3)
				assert.Equal(t, "2023-01-01T00:00:00Z", result[0].Args[0])
				assert.Equal(t, "2023-12-31T23:59:59Z", result[0].Args[1])
				assert.Equal(t, []string{"system", "bot"}, result[0].Args[2])

				// Check joins
				expectedJoins := []r3sql.SQLColumn{r3sql.SQLColumn("user")}
				assert.ElementsMatch(t, expectedJoins, result[0].Joins)
			},
		},
		{
			name:     "multiple independent filters",
			scenario: "Multiple independent filter conditions",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("status"),
					Operator: r3.OperatorEq,
					Value:    "published",
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("author.verified"),
					Operator: r3.OperatorEq,
					Value:    true,
				},
				&r3.FilterSpec{
					Or: r3.Filters{
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("priority"),
							Operator: r3.OperatorGte,
							Value:    5,
						},
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("featured"),
							Operator: r3.OperatorEq,
							Value:    true,
						},
					},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				require.Len(t, result, 3)

				// First filter: status = 'published'
				assert.Equal(t, "status = ?", result[0].Clause)
				require.Len(t, result[0].Args, 1)
				assert.Equal(t, "published", result[0].Args[0])

				// Second filter: author.verified = true
				assert.Equal(t, "author.verified = ?", result[1].Clause)
				require.Len(t, result[1].Args, 1)
				assert.Equal(t, true, result[1].Args[0])
				require.Len(t, result[1].Joins, 1)
				assert.Equal(t, r3sql.SQLColumn("author"), result[1].Joins[0])

				// Third filter: (priority >= 5 OR featured = true)
				assert.Equal(t, "(priority >= ? OR featured = ?)", result[2].Clause)
				require.Len(t, result[2].Args, 2)
				assert.Equal(t, 5, result[2].Args[0])
				assert.Equal(t, true, result[2].Args[1])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.FiltersToSQLClauses(tt.input)
			require.NoError(t, err, "Scenario: %s", tt.scenario)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestFiltersToSQLClauses_CombinedJoins(t *testing.T) {
	// Test that joins are properly combined across multiple clauses
	tests := []struct {
		name        string
		input       r3.Filters
		expectedLen int
		validate    func(t *testing.T, result r3sql.SQLClauses)
	}{
		{
			name: "multiple clauses with different joins",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("user.name"),
					Operator: r3.OperatorEq,
					Value:    "John",
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("profile.age"),
					Operator: r3.OperatorGte,
					Value:    18,
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("address.city"),
					Operator: r3.OperatorEq,
					Value:    "NYC",
				},
			},
			expectedLen: 3,
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				allJoins := result.Joins()
				expectedJoins := []r3sql.SQLColumn{
					r3sql.SQLColumn("user"),
					r3sql.SQLColumn("profile"),
					r3sql.SQLColumn("address"),
				}
				assert.ElementsMatch(t, expectedJoins, allJoins)
			},
		},
		{
			name: "multiple clauses with overlapping joins",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("user.name"),
					Operator: r3.OperatorEq,
					Value:    "John",
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("user.email"),
					Operator: r3.OperatorEq,
					Value:    "john@example.com",
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("profile.verified"),
					Operator: r3.OperatorEq,
					Value:    true,
				},
			},
			expectedLen: 3,
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				allJoins := result.Joins()
				// Should be deduplicated by quick.Append
				expectedJoins := []r3sql.SQLColumn{
					r3sql.SQLColumn("user"),
					r3sql.SQLColumn("profile"),
				}
				assert.Equal(t, expectedJoins, allJoins)
			},
		},
		{
			name: "no joins scenario",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("id"),
					Operator: r3.OperatorGt,
					Value:    100,
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("status"),
					Operator: r3.OperatorEq,
					Value:    "active",
				},
			},
			expectedLen: 2,
			validate: func(t *testing.T, result r3sql.SQLClauses) {
				allJoins := result.Joins()
				assert.Empty(t, allJoins)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.FiltersToSQLClauses(tt.input)
			require.NoError(t, err)
			require.Len(t, result, tt.expectedLen)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestFiltersToSQLClauses_ErrorHandling(t *testing.T) {
	// Test error scenarios
	tests := []struct {
		name        string
		input       r3.Filters
		expectError bool
		description string
	}{
		{
			name: "unsupported operator for NULL",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("field"),
					Operator: r3.OperatorGt,
					Value:    nil,
				},
			},
			expectError: true,
			description: "Should error on unsupported operator with nil value",
		},
		{
			name: "valid filters should not error",
			input: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("valid_field"),
					Operator: r3.OperatorEq,
					Value:    "valid_value",
				},
			},
			expectError: false,
			description: "Valid filters should not cause errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.FiltersToSQLClauses(tt.input)

			if tt.expectError {
				require.Error(t, err, tt.description)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err, tt.description)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestFiltersToSQLClauses_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       r3.Filters
		expectError bool
		description string
	}{
		{
			name:        "empty filters slice",
			input:       r3.Filters{},
			expectError: false,
			description: "Empty filters should return empty result without error",
		},
		{
			name:        "nil filters slice",
			input:       nil,
			expectError: false,
			description: "Nil filters should return nil result without error",
		},
		{
			name: "deeply nested filters",
			input: r3.Filters{
				&r3.FilterSpec{
					And: r3.Filters{
						&r3.FilterSpec{
							Or: r3.Filters{
								&r3.FilterSpec{
									And: r3.Filters{
										&r3.FilterSpec{
											Field:    r3.NewFieldSpec("deep.field"),
											Operator: r3.OperatorEq,
											Value:    "value",
										},
									},
								},
							},
						},
					},
				},
			},
			expectError: false,
			description: "Deeply nested filters should be handled correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.FiltersToSQLClauses(tt.input)

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

func BenchmarkFiltersToSQLClauses(b *testing.B) {
	// Create a complex set of filters for benchmarking
	filters := r3.Filters{
		&r3.FilterSpec{
			And: r3.Filters{
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("user.status"),
					Operator: r3.OperatorIn,
					Value:    []string{"active", "pending", "verified"},
				},
				&r3.FilterSpec{
					Or: r3.Filters{
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("subscription.tier"),
							Operator: r3.OperatorEq,
							Value:    "premium",
						},
						&r3.FilterSpec{
							Field:    r3.NewFieldSpec("trial.active"),
							Operator: r3.OperatorEq,
							Value:    true,
						},
					},
				},
				&r3.FilterSpec{
					Field:    r3.NewFieldSpec("created_at"),
					Operator: r3.OperatorGte,
					Value:    "2023-01-01T00:00:00Z",
				},
			},
		},
		&r3.FilterSpec{
			Field:    r3.NewFieldSpec("deleted_at"),
			Operator: r3.OperatorEq,
			Value:    nil,
		},
		&r3.FilterSpec{
			Field:    r3.NewFieldSpec("profile.verified"),
			Operator: r3.OperatorEq,
			Value:    true,
		},
	}

	b.ResetTimer()
	for range b.N {
		_, err := r3sql.FiltersToSQLClauses(filters)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestSQLClauseOperator_String(t *testing.T) {
	tests := []struct {
		name     string
		operator r3sql.SQLClauseOperator
		expected string
	}{
		{
			name:     "equals operator",
			operator: r3sql.SQLClauseOperatorEq,
			expected: "=",
		},
		{
			name:     "not equals operator",
			operator: r3sql.SQLClauseOperatorNe,
			expected: "!=",
		},
		{
			name:     "greater than operator",
			operator: r3sql.SQLClauseOperatorGt,
			expected: ">",
		},
		{
			name:     "LIKE operator",
			operator: r3sql.SQLClauseOperatorLike,
			expected: "LIKE",
		},
		{
			name:     "IN operator",
			operator: r3sql.SQLClauseOperatorIn,
			expected: "IN",
		},
		{
			name:     "ILIKE operator",
			operator: r3sql.SQLClauseOperatorILike,
			expected: "ILIKE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.operator.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSQLClauseOperator_ToSQL(t *testing.T) {
	tests := []struct {
		name     string
		operator r3sql.SQLClauseOperator
		expected string
	}{
		{
			name:     "equals operator",
			operator: r3sql.SQLClauseOperatorEq,
			expected: "=",
		},
		{
			name:     "LIKE operator",
			operator: r3sql.SQLClauseOperatorLike,
			expected: "LIKE",
		},
		{
			name:     "NOT IN operator",
			operator: r3sql.SQLClauseOperatorNotIn,
			expected: "NOT IN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.operator.ToSQL()
			assert.Equal(t, tt.expected, result)
		})
	}
}
