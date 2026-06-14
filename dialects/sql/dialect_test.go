package r3sql_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldToSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    *r3.FieldSpec
		expected string
	}{
		{
			name:     "simple field",
			input:    r3.NewFieldSpec("id"),
			expected: "id",
		},
		{
			name:     "nested field",
			input:    r3.NewFieldSpec("user.name"),
			expected: "user.name",
		},
		{
			name:     "complex field path",
			input:    r3.NewFieldSpec("orders.items.product.name"),
			expected: "orders.items.product.name",
		},
		{
			name:     "field with underscores",
			input:    r3.NewFieldSpec("user_profile_id"),
			expected: "user_profile_id",
		},
		{
			name:     "empty field",
			input:    r3.NewFieldSpec(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3sql.FieldToSQL(tt.input)
			assert.Equal(t, tt.expected, result.String())
		})
	}
}

func TestSQLToField(t *testing.T) {
	tests := []struct {
		name     string
		input    r3sql.SQLColumn
		expected string
	}{
		{
			name:     "SQLColumn value",
			input:    r3sql.SQLColumn("user_id"),
			expected: "user_id",
		},
		{
			name:     "complex field path",
			input:    r3sql.SQLColumn("orders.items.product.name"),
			expected: "orders.items.product.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3sql.SQLToField(tt.input)
			require.NotNil(t, result)
			assert.Equal(t, tt.expected, result.String())
		})
	}
}

func TestFilterToSQL(t *testing.T) {
	tests := []struct {
		name        string
		input       *r3.FilterSpec
		expectError bool
		validate    func(t *testing.T, result r3sql.SQLClause)
	}{
		{
			name: "simple equality filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("id"),
				Operator: r3.OperatorEq,
				Value:    123,
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"id" = ?`, result.Clause)
				require.Len(t, result.Args, 1)
				assert.Equal(t, 123, result.Args[0])
				assert.Empty(t, result.Joins)
			},
		},
		{
			name: "not equal filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("status"),
				Operator: r3.OperatorNe,
				Value:    "inactive",
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"status" != ?`, result.Clause)
				require.Len(t, result.Args, 1)
				assert.Equal(t, "inactive", result.Args[0])
			},
		},
		{
			name: "greater than filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("age"),
				Operator: r3.OperatorGt,
				Value:    18,
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"age" > ?`, result.Clause)
				require.Len(t, result.Args, 1)
				assert.Equal(t, 18, result.Args[0])
			},
		},
		{
			name: "LIKE filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("name"),
				Operator: r3.OperatorLike,
				Value:    "John%",
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"name" LIKE ?`, result.Clause)
				require.Len(t, result.Args, 1)
				assert.Equal(t, "John%", result.Args[0])
			},
		},
		{
			name: "IN filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("category"),
				Operator: r3.OperatorIn,
				Value:    []string{"electronics", "books"},
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"category" IN (?, ?)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.Equal(t, "electronics", result.Args[0])
				assert.Equal(t, "books", result.Args[1])
			},
		},
		{
			name: "NULL equality filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("deleted_at"),
				Operator: r3.OperatorEq,
				Value:    nil,
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"deleted_at" IS NULL`, result.Clause)
				assert.Empty(t, result.Args)
			},
		},
		{
			name: "NULL inequality filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("deleted_at"),
				Operator: r3.OperatorNe,
				Value:    nil,
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"deleted_at" IS NOT NULL`, result.Clause)
				assert.Empty(t, result.Args)
			},
		},
		{
			name: "field with joins",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("user.profile.name"),
				Operator: r3.OperatorEq,
				Value:    "John",
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"user"."profile"."name" = ?`, result.Clause)
				require.Len(t, result.Args, 1)
				assert.Equal(t, "John", result.Args[0])
				require.Len(t, result.Joins, 1)
				assert.Equal(t, `"user"`, result.Joins[0].String())
			},
		},
		{
			name: "AND filter group",
			input: &r3.FilterSpec{
				And: r3.Filters{
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("age"),
						Operator: r3.OperatorGte,
						Value:    18,
					},
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("status"),
						Operator: r3.OperatorEq,
						Value:    "active",
					},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("age" >= ? AND "status" = ?)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.Equal(t, 18, result.Args[0])
				assert.Equal(t, "active", result.Args[1])
			},
		},
		{
			name: "OR filter group",
			input: &r3.FilterSpec{
				Or: r3.Filters{
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("type"),
						Operator: r3.OperatorEq,
						Value:    "premium",
					},
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("vip"),
						Operator: r3.OperatorEq,
						Value:    true,
					},
				},
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("type" = ? OR "vip" = ?)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.Equal(t, "premium", result.Args[0])
				assert.Equal(t, true, result.Args[1])
			},
		},
		{
			name: "complex nested filter",
			input: &r3.FilterSpec{
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
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("category" = ? AND ("price" < ? OR "on_sale" = ?))`, result.Clause)
				require.Len(t, result.Args, 3)
				assert.Equal(t, "electronics", result.Args[0])
				assert.Equal(t, 100, result.Args[1])
				assert.Equal(t, true, result.Args[2])
			},
		},
		{
			name: "exists operator",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("email"),
				Operator: r3.OperatorExists,
				Value:    true,
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `"email" IS NOT NULL`, result.Clause)
				assert.Empty(t, result.Args)
			},
		},
		{
			name: "between inclusive",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("age"),
				Operator: r3.OperatorBetween,
				Value:    []int{18, 65},
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("age" >= ? AND "age" <= ?)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.Equal(t, 18, result.Args[0])
				assert.Equal(t, 65, result.Args[1])
			},
		},
		{
			name: "between exclusive",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("price"),
				Operator: r3.OperatorBetweenEx,
				Value:    []float64{9.99, 99.99},
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("price" > ? AND "price" < ?)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.InDelta(t, 9.99, result.Args[0], 0.001)
				assert.InDelta(t, 99.99, result.Args[1], 0.001)
			},
		},
		{
			name: "between exclusive-inclusive",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("score"),
				Operator: r3.OperatorBetweenExInc,
				Value:    []int{0, 100},
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("score" > ? AND "score" <= ?)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.Equal(t, 0, result.Args[0])
				assert.Equal(t, 100, result.Args[1])
			},
		},
		{
			name: "between inclusive-exclusive",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("weight"),
				Operator: r3.OperatorBetweenIncEx,
				Value:    []int{50, 200},
			},
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("weight" >= ? AND "weight" < ?)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.Equal(t, 50, result.Args[0])
				assert.Equal(t, 200, result.Args[1])
			},
		},
		{
			name: "between with non-slice value",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("age"),
				Operator: r3.OperatorBetween,
				Value:    42,
			},
			expectError: true,
		},
		{
			name: "between with wrong element count",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("age"),
				Operator: r3.OperatorBetween,
				Value:    []int{18},
			},
			expectError: true,
		},
		{
			name: "unsupported operator for NULL",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("field"),
				Operator: r3.OperatorGt,
				Value:    nil,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.FilterToSQL(tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// TestFilterToSQL_SQLInjection verifies that SQL injection attempts are rejected.
func TestFilterToSQL_SQLInjection(t *testing.T) {
	injectionAttempts := []struct {
		name  string
		field string
	}{
		{"semicolon injection", "id; DROP TABLE users--"},
		{"union injection", "1=1) UNION SELECT password FROM users--"},
		{"comment injection", "id--"},
		{"subquery injection", "(SELECT 1)"},
		{"space in name", "col name"},
		{"single quote", "col'name"},
		{"wildcard", "table.*"},
		{"equals injection", "1=1"},
		{"comma injection", "a,b"},
	}

	for _, tt := range injectionAttempts {
		t.Run(tt.name+"/equality", func(t *testing.T) {
			_, err := r3sql.FilterToSQL(&r3.FilterSpec{
				Field:    r3.NewFieldSpec(tt.field),
				Operator: r3.OperatorEq,
				Value:    "anything",
			})
			require.Error(t, err, "filter with field %q should be rejected", tt.field)
		})

		t.Run(tt.name+"/null", func(t *testing.T) {
			_, err := r3sql.FilterToSQL(&r3.FilterSpec{
				Field:    r3.NewFieldSpec(tt.field),
				Operator: r3.OperatorEq,
				Value:    nil,
			})
			require.Error(t, err, "null filter with field %q should be rejected", tt.field)
		})
	}
}

// TestSortToSQL_SQLInjection verifies that SQL injection attempts in sort columns are rejected.
func TestSortToSQL_SQLInjection(t *testing.T) {
	injectionAttempts := []struct {
		name  string
		field string
	}{
		{"semicolon injection", "id; DROP TABLE users--"},
		{"subquery injection", "(SELECT 1)"},
		{"comment injection", "id--"},
		{"space in name", "col name"},
	}

	for _, tt := range injectionAttempts {
		t.Run(tt.name, func(t *testing.T) {
			_, err := r3sql.SortToSQL(&r3.SortSpec{
				Column:    r3.NewFieldSpec(tt.field),
				Direction: r3.SortDirectionAsc,
			})
			require.Error(t, err, "sort with column %q should be rejected", tt.field)
		})
	}
}

// TestSafeColumnExpr tests the SafeColumnExpr function directly.
func TestSafeColumnExpr(t *testing.T) {
	tests := []struct {
		name        string
		input       *r3.FieldSpec
		expected    string
		expectError bool
	}{
		{name: "simple", input: r3.NewFieldSpec("id"), expected: `"id"`},
		{name: "underscore", input: r3.NewFieldSpec("user_name"), expected: `"user_name"`},
		{name: "dotted", input: r3.NewFieldSpec("user.name"), expected: `"user"."name"`},
		{name: "deep dotted", input: r3.NewFieldSpec("a.b.c"), expected: `"a"."b"."c"`},
		{name: "nil field", input: nil, expectError: true},
		{name: "empty", input: r3.NewFieldSpec(""), expectError: true},
		{name: "injection", input: r3.NewFieldSpec("id; DROP TABLE"), expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.SafeColumnExpr(tt.input)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestQuoteIdentifier tests the QuoteIdentifier function.
func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "simple", input: "id", expected: `"id"`},
		{name: "with underscore", input: "user_name", expected: `"user_name"`},
		{name: "with embedded quote", input: `col"name`, expected: `"col""name"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, r3sql.QuoteIdentifier(tt.input))
		})
	}
}

func TestOperatorToSQL(t *testing.T) {
	tests := []struct {
		name        string
		input       r3.FilterOperatorSpec
		expected    r3sql.SQLClauseOperator
		expectError bool
	}{
		{name: "OperatorEq", input: r3.OperatorEq, expected: r3sql.SQLClauseOperatorEq},
		{name: "OperatorNe", input: r3.OperatorNe, expected: r3sql.SQLClauseOperatorNe},
		{name: "OperatorGt", input: r3.OperatorGt, expected: r3sql.SQLClauseOperatorGt},
		{name: "OperatorGte", input: r3.OperatorGte, expected: r3sql.SQLClauseOperatorGte},
		{name: "OperatorLt", input: r3.OperatorLt, expected: r3sql.SQLClauseOperatorLt},
		{name: "OperatorLte", input: r3.OperatorLte, expected: r3sql.SQLClauseOperatorLte},
		{name: "OperatorLike", input: r3.OperatorLike, expected: r3sql.SQLClauseOperatorLike},
		{name: "OperatorNotLike", input: r3.OperatorNotLike, expected: r3sql.SQLClauseOperatorNotLike},
		{name: "OperatorILike", input: r3.OperatorILike, expected: r3sql.SQLClauseOperatorILike},
		{name: "OperatorIn", input: r3.OperatorIn, expected: r3sql.SQLClauseOperatorIn},
		{name: "OperatorNotIn", input: r3.OperatorNotIn, expected: r3sql.SQLClauseOperatorNotIn},
		{name: "OperatorBetween - not implemented", input: r3.OperatorBetween, expectError: true},
		{name: "OperatorExists - not implemented", input: r3.OperatorExists, expectError: true},
		{name: "OperatorUnspecified", input: r3.OperatorUnspecified, expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.OperatorToSQL(tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSQLToOperator(t *testing.T) {
	tests := []struct {
		name        string
		input       r3sql.SQLClauseOperator
		expected    r3.FilterOperatorSpec
		expectError bool
	}{
		{name: "Eq", input: r3sql.SQLClauseOperatorEq, expected: r3.OperatorEq},
		{name: "Gt", input: r3sql.SQLClauseOperatorGt, expected: r3.OperatorGt},
		{name: "Like", input: r3sql.SQLClauseOperatorLike, expected: r3.OperatorLike},
		{name: "NotIn", input: r3sql.SQLClauseOperatorNotIn, expected: r3.OperatorNotIn},
		{name: "unsupported", input: r3sql.SQLClauseOperator("CUSTOM_OP"), expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.SQLToOperator(tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSortToSQL(t *testing.T) {
	tests := []struct {
		name        string
		input       *r3.SortSpec
		expected    string
		expectError bool
	}{
		{
			name: "ascending sort",
			input: &r3.SortSpec{
				Column:    r3.NewFieldSpec("name"),
				Direction: r3.SortDirectionAsc,
			},
			expected: `"name" ASC`,
		},
		{
			name: "descending sort",
			input: &r3.SortSpec{
				Column:    r3.NewFieldSpec("created_at"),
				Direction: r3.SortDirectionDesc,
			},
			expected: `"created_at" DESC`,
		},
		{
			name: "sort with nulls first",
			input: &r3.SortSpec{
				Column:        r3.NewFieldSpec("priority"),
				Direction:     r3.SortDirectionDesc,
				NullsPosition: r3.NullsPositionFirst,
			},
			expected: `"priority" DESC NULLS FIRST`,
		},
		{
			name: "sort with nulls last",
			input: &r3.SortSpec{
				Column:        r3.NewFieldSpec("score"),
				Direction:     r3.SortDirectionAsc,
				NullsPosition: r3.NullsPositionLast,
			},
			expected: `"score" ASC NULLS LAST`,
		},
		{
			name: "complex field sort",
			input: &r3.SortSpec{
				Column:    r3.NewFieldSpec("user.profile.last_login"),
				Direction: r3.SortDirectionDesc,
			},
			expected: `"user"."profile"."last_login" DESC`,
		},
		{
			name:        "nil sort spec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.SortToSQL(tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, string(result))
			}
		})
	}
}

func TestSQLToSort(t *testing.T) {
	tests := []struct {
		name        string
		input       r3sql.SQLSort
		expectError bool
		validate    func(t *testing.T, result *r3.SortSpec)
	}{
		{
			name:  "simple ASC",
			input: r3sql.SQLSort("name ASC"),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "name", result.Column.String())
				assert.Equal(t, r3.SortDirectionAsc, result.Direction)
			},
		},
		{
			name:  "simple DESC",
			input: r3sql.SQLSort("created_at DESC"),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "created_at", result.Column.String())
				assert.Equal(t, r3.SortDirectionDesc, result.Direction)
			},
		},
		{
			name:  "field only",
			input: r3sql.SQLSort("id"),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "id", result.Column.String())
				assert.Equal(t, r3.SortDirectionUnspecified, result.Direction)
			},
		},
		{
			name:  "sort with nulls first",
			input: r3sql.SQLSort("status DESC NULLS FIRST"),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "status", result.Column.String())
				assert.Equal(t, r3.SortDirectionDesc, result.Direction)
				assert.Equal(t, r3.NullsPositionFirst, result.NullsPosition)
			},
		},
		{
			name:  "sort with nulls last",
			input: r3sql.SQLSort("updated_at ASC NULLS LAST"),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "updated_at", result.Column.String())
				assert.Equal(t, r3.SortDirectionAsc, result.Direction)
				assert.Equal(t, r3.NullsPositionLast, result.NullsPosition)
			},
		},
		{
			name:        "empty string",
			input:       r3sql.SQLSort(""),
			expectError: true,
		},
		{
			name:        "whitespace only",
			input:       r3sql.SQLSort("   "),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.SQLToSort(tt.input)

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

func TestFilterToSQL_Integration(t *testing.T) {
	// Test realistic scenarios combining multiple features
	tests := []struct {
		name     string
		scenario string
		input    *r3.FilterSpec
		validate func(t *testing.T, result r3sql.SQLClause)
	}{
		{
			name:     "user authentication query",
			scenario: "Find active user with specific email and valid login",
			input: &r3.FilterSpec{
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
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(t, `("email" = ? AND "status" = ? AND "last_login" IS NOT NULL)`, result.Clause)
				require.Len(t, result.Args, 2)
				assert.Equal(t, "user@example.com", result.Args[0])
				assert.Equal(t, "active", result.Args[1])
			},
		},
		{
			name:     "product search with joins",
			scenario: "Find products in categories with specific properties",
			input: &r3.FilterSpec{
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
			validate: func(t *testing.T, result r3sql.SQLClause) {
				assert.Equal(
					t,
					`("category"."name" IN (?, ?) AND ("price" < ? OR "discount"."active" = ?))`,
					result.Clause,
				)
				require.Len(t, result.Args, 4)
				assert.Equal(t, "electronics", result.Args[0])
				assert.Equal(t, "computers", result.Args[1])
				assert.Equal(t, 1000, result.Args[2])
				assert.Equal(t, true, result.Args[3])

				// Joins should be quoted table names
				expectedJoins := []r3sql.SQLColumn{r3sql.SQLColumn(`"category"`), r3sql.SQLColumn(`"discount"`)}
				assert.ElementsMatch(t, expectedJoins, result.Joins)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3sql.FilterToSQL(tt.input)
			require.NoError(t, err, "Scenario: %s", tt.scenario)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func BenchmarkFilterToSQL(b *testing.B) {
	// Create a complex filter for benchmarking
	filter := &r3.FilterSpec{
		And: r3.Filters{
			&r3.FilterSpec{
				Field:    r3.NewFieldSpec("user.profile.status"),
				Operator: r3.OperatorEq,
				Value:    "active",
			},
			&r3.FilterSpec{
				Or: r3.Filters{
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("subscription.type"),
						Operator: r3.OperatorIn,
						Value:    []string{"premium", "enterprise"},
					},
					&r3.FilterSpec{
						Field:    r3.NewFieldSpec("trial.active"),
						Operator: r3.OperatorEq,
						Value:    true,
					},
				},
			},
			&r3.FilterSpec{
				Field:    r3.NewFieldSpec("deleted_at"),
				Operator: r3.OperatorEq,
				Value:    nil,
			},
		},
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := r3sql.FilterToSQL(filter)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestPaginationToSQL(t *testing.T) {
	tests := []struct {
		name     string
		input    *r3.PaginationSpec
		expected *r3sql.SQLPagination
	}{
		{
			name:     "valid pagination",
			input:    r3.NewPaginationSpec(2, 25),
			expected: r3sql.NewSQLPagination(25, 25),
		},
		{
			name:     "page 1",
			input:    r3.NewPaginationSpec(1, 50),
			expected: r3sql.NewSQLPagination(50, 0),
		},
		{
			name:     "default pagination",
			input:    r3.DefaultPagination(),
			expected: r3sql.NewSQLPagination(r3.PageSizeDefault, 0),
		},
		{
			name:     "nil pagination",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3sql.PaginationToSQL(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Limit, result.Limit)
				assert.Equal(t, tt.expected.Offset, result.Offset)
			}
		})
	}
}

func TestSQLToPagination(t *testing.T) {
	tests := []struct {
		name  string
		input *r3sql.SQLPagination
		isNil bool
	}{
		{
			name:  "valid SQL pagination",
			input: r3sql.NewSQLPagination(25, 25),
		},
		{
			name:  "nil input",
			input: nil,
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3sql.SQLToPagination(tt.input)
			if tt.isNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
			}
		})
	}
}

func TestPagination_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		pageNum  int
		pageSize int
	}{
		{name: "page 1 size 10", pageNum: 1, pageSize: 10},
		{name: "page 2 size 25", pageNum: 2, pageSize: 25},
		{name: "page 5 size 50", pageNum: 5, pageSize: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create original PaginationSpec
			original := r3.NewPaginationSpec(tt.pageNum, tt.pageSize)

			// Convert to SQL
			sqlPagination := r3sql.PaginationToSQL(original)
			require.NotNil(t, sqlPagination)

			// Convert back to PaginationSpec
			result := r3sql.SQLToPagination(sqlPagination)
			require.NotNil(t, result)

			// Verify round-trip
			assert.Equal(t, tt.pageNum, result.GetPageNum())
			assert.Equal(t, tt.pageSize, result.GetPageSize())
		})
	}
}

// TestFilterToSQL_InExpansion covers IN / NOT IN placeholder expansion and the
// degenerate (empty-set) and scalar edge cases. A single "col IN ?" bound to a
// slice is not expanded by database/sql, so the dialect must emit one
// placeholder per value (C1).
func TestFilterToSQL_InExpansion(t *testing.T) {
	tests := []struct {
		name       string
		filter     *r3.FilterSpec
		wantClause string
		wantArgs   []any
	}{
		{
			name:       "IN with multiple values",
			filter:     r3.In("status", []string{"active", "pending"}),
			wantClause: `"status" IN (?, ?)`,
			wantArgs:   []any{"active", "pending"},
		},
		{
			name:       "IN with a single value",
			filter:     r3.In("status", []string{"active"}),
			wantClause: `"status" IN (?)`,
			wantArgs:   []any{"active"},
		},
		{
			name:       "NOT IN with multiple values",
			filter:     r3.NotIn("status", []int{1, 2, 3}),
			wantClause: `"status" NOT IN (?, ?, ?)`,
			wantArgs:   []any{1, 2, 3},
		},
		{
			name:       "empty IN collapses to constant false",
			filter:     r3.In("status", []string{}),
			wantClause: "1=0",
			wantArgs:   nil,
		},
		{
			name:       "empty NOT IN collapses to constant true",
			filter:     r3.NotIn("status", []string{}),
			wantClause: "1=1",
			wantArgs:   nil,
		},
		{
			name:       "scalar value passed to IN is treated as a single-element set",
			filter:     r3.In("status", "active"),
			wantClause: `"status" IN (?)`,
			wantArgs:   []any{"active"},
		},
		{
			name:       "byte slice is a scalar, not a list",
			filter:     r3.In("data", []byte{0x01, 0x02}),
			wantClause: `"data" IN (?)`,
			wantArgs:   []any{[]byte{0x01, 0x02}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause, err := r3sql.FilterToSQL(tt.filter)
			require.NoError(t, err)
			assert.Equal(t, tt.wantClause, clause.Clause)
			assert.Equal(t, tt.wantArgs, clause.Args)
		})
	}
}
