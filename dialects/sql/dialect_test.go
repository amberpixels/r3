package r3sql_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLDialector_ToSQLColumn(t *testing.T) {
	dialector := &r3sql.SQLDialector{}

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
			result := dialector.ToSQLColumn(tt.input)
			assert.Equal(t, tt.expected, result.String())
		})
	}
}

func TestSQLDialector_TranslateFieldSpec(t *testing.T) {
	dialector := &r3sql.SQLDialector{}

	tests := []struct {
		name        string
		input       *r3.FieldSpec
		expected    string
		expectError bool
	}{
		{
			name:     "valid field spec",
			input:    r3.NewFieldSpec("user_id"),
			expected: "user_id",
		},
		{
			name:     "field with dots",
			input:    r3.NewFieldSpec("profile.user.name"),
			expected: "profile.user.name",
		},
		{
			name:     "empty field",
			input:    r3.NewFieldSpec(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateFieldSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				sqlCol, ok := result.(r3sql.SQLColumn)
				require.True(t, ok)
				assert.Equal(t, tt.expected, sqlCol.String())
			}
		})
	}
}

func TestSQLDialector_TranslateFilterSpec(t *testing.T) {
	dialector := &r3sql.SQLDialector{}

	tests := []struct {
		name        string
		input       *r3.FilterSpec
		expectError bool
		validate    func(t *testing.T, result r3.DialectValue)
	}{
		{
			name: "simple equality filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("id"),
				Operator: r3.OperatorEq,
				Value:    123,
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "id = ?", clause.Clause)
				require.Len(t, clause.Args, 1)
				assert.Equal(t, 123, clause.Args[0])
				assert.Empty(t, clause.Joins)
			},
		},
		{
			name: "not equal filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("status"),
				Operator: r3.OperatorNe,
				Value:    "inactive",
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "status != ?", clause.Clause)
				require.Len(t, clause.Args, 1)
				assert.Equal(t, "inactive", clause.Args[0])
			},
		},
		{
			name: "greater than filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("age"),
				Operator: r3.OperatorGt,
				Value:    18,
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "age > ?", clause.Clause)
				require.Len(t, clause.Args, 1)
				assert.Equal(t, 18, clause.Args[0])
			},
		},
		{
			name: "LIKE filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("name"),
				Operator: r3.OperatorLike,
				Value:    "John%",
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "name LIKE ?", clause.Clause)
				require.Len(t, clause.Args, 1)
				assert.Equal(t, "John%", clause.Args[0])
			},
		},
		{
			name: "IN filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("category"),
				Operator: r3.OperatorIn,
				Value:    []string{"electronics", "books"},
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "category IN ?", clause.Clause)
				require.Len(t, clause.Args, 1)
				assert.Equal(t, []string{"electronics", "books"}, clause.Args[0])
			},
		},
		{
			name: "NULL equality filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("deleted_at"),
				Operator: r3.OperatorEq,
				Value:    nil,
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "deleted_at IS NULL", clause.Clause)
				assert.Empty(t, clause.Args)
			},
		},
		{
			name: "NULL inequality filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("deleted_at"),
				Operator: r3.OperatorNe,
				Value:    nil,
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "deleted_at IS NOT NULL", clause.Clause)
				assert.Empty(t, clause.Args)
			},
		},
		{
			name: "field with joins",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("user.profile.name"),
				Operator: r3.OperatorEq,
				Value:    "John",
			},
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "user.profile.name = ?", clause.Clause)
				require.Len(t, clause.Args, 1)
				assert.Equal(t, "John", clause.Args[0])
				require.Len(t, clause.Joins, 1)
				assert.Equal(t, "user", clause.Joins[0].String())
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
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "(age >= ? AND status = ?)", clause.Clause)
				require.Len(t, clause.Args, 2)
				assert.Equal(t, 18, clause.Args[0])
				assert.Equal(t, "active", clause.Args[1])
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
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "(type = ? OR vip = ?)", clause.Clause)
				require.Len(t, clause.Args, 2)
				assert.Equal(t, "premium", clause.Args[0])
				assert.Equal(t, true, clause.Args[1])
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
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "(category = ? AND (price < ? OR on_sale = ?))", clause.Clause)
				require.Len(t, clause.Args, 3)
				assert.Equal(t, "electronics", clause.Args[0])
				assert.Equal(t, 100, clause.Args[1])
				assert.Equal(t, true, clause.Args[2])
			},
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
			result, err := dialector.TranslateFilterSpec(tt.input)

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

func TestSQLDialector_TranslateFilterOperatorSpec(t *testing.T) {
	dialector := &r3sql.SQLDialector{}

	tests := []struct {
		name        string
		input       *r3.FilterOperatorSpec
		expected    r3sql.SQLClauseOperator
		expectError bool
	}{
		{
			name:     "OperatorEq",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorEq; return &op }(),
			expected: r3sql.SQLClauseOperatorEq,
		},
		{
			name:     "OperatorNe",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorNe; return &op }(),
			expected: r3sql.SQLClauseOperatorNe,
		},
		{
			name:     "OperatorGt",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorGt; return &op }(),
			expected: r3sql.SQLClauseOperatorGt,
		},
		{
			name:     "OperatorGte",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorGte; return &op }(),
			expected: r3sql.SQLClauseOperatorGte,
		},
		{
			name:     "OperatorLt",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorLt; return &op }(),
			expected: r3sql.SQLClauseOperatorLt,
		},
		{
			name:     "OperatorLte",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorLte; return &op }(),
			expected: r3sql.SQLClauseOperatorLte,
		},
		{
			name:     "OperatorLike",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorLike; return &op }(),
			expected: r3sql.SQLClauseOperatorLike,
		},
		{
			name:     "OperatorNotLike",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorNotLike; return &op }(),
			expected: r3sql.SQLClauseOperatorNotLike,
		},
		{
			name:     "OperatorILike",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorILike; return &op }(),
			expected: r3sql.SQLClauseOperatorILike,
		},
		{
			name:     "OperatorIn",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorIn; return &op }(),
			expected: r3sql.SQLClauseOperatorIn,
		},
		{
			name:     "OperatorNotIn",
			input:    func() *r3.FilterOperatorSpec { op := r3.OperatorNotIn; return &op }(),
			expected: r3sql.SQLClauseOperatorNotIn,
		},
		{
			name:        "OperatorBetween - not implemented",
			input:       func() *r3.FilterOperatorSpec { op := r3.OperatorBetween; return &op }(),
			expectError: true,
		},
		{
			name:        "OperatorExists - not implemented",
			input:       func() *r3.FilterOperatorSpec { op := r3.OperatorExists; return &op }(),
			expectError: true,
		},
		{
			name:        "OperatorUnspecified",
			input:       func() *r3.FilterOperatorSpec { op := r3.OperatorUnspecified; return &op }(),
			expectError: true,
		},
		{
			name:        "nil operator",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateFilterOperatorSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				sqlOp, ok := result.(r3sql.SQLClauseOperator)
				require.True(t, ok)
				assert.Equal(t, tt.expected, sqlOp)
			}
		})
	}
}

func TestSQLDialector_TranslateSortSpec(t *testing.T) {
	dialector := &r3sql.SQLDialector{}

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
			expected: "name ASC",
		},
		{
			name: "descending sort",
			input: &r3.SortSpec{
				Column:    r3.NewFieldSpec("created_at"),
				Direction: r3.SortDirectionDesc,
			},
			expected: "created_at DESC",
		},
		{
			name: "sort with nulls first",
			input: &r3.SortSpec{
				Column:        r3.NewFieldSpec("priority"),
				Direction:     r3.SortDirectionDesc,
				NullsPosition: r3.NullsPositionFirst,
			},
			expected: "priority DESC NULLS FIRST",
		},
		{
			name: "sort with nulls last",
			input: &r3.SortSpec{
				Column:        r3.NewFieldSpec("score"),
				Direction:     r3.SortDirectionAsc,
				NullsPosition: r3.NullsPositionLast,
			},
			expected: "score ASC NULLS LAST",
		},
		{
			name: "complex field sort",
			input: &r3.SortSpec{
				Column:    r3.NewFieldSpec("user.profile.last_login"),
				Direction: r3.SortDirectionDesc,
			},
			expected: "user.profile.last_login DESC",
		},
		{
			name:        "nil sort spec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateSortSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				sqlSort, ok := result.(r3sql.SQLSort)
				require.True(t, ok)
				assert.Equal(t, tt.expected, string(sqlSort))
			}
		})
	}
}

func TestSQLInboundDialector_TranslateIntoFieldSpec(t *testing.T) {
	dialector := &r3sql.SQLInboundDialector{}

	tests := []struct {
		name        string
		input       r3.DialectValue
		expected    string
		expectError bool
	}{
		{
			name:     "SQLColumn value",
			input:    r3sql.SQLColumn("user_id"),
			expected: "user_id",
		},
		{
			name:     "SQLColumn pointer",
			input:    func() *r3sql.SQLColumn { col := r3sql.SQLColumn("profile_name"); return &col }(),
			expected: "profile_name",
		},
		{
			name:     "string value",
			input:    "status",
			expected: "status",
		},
		{
			name:     "complex field path",
			input:    r3sql.SQLColumn("orders.items.product.name"),
			expected: "orders.items.product.name",
		},
		{
			name:        "invalid type",
			input:       123,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoFieldSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected, result.String())
			}
		})
	}
}

func TestSQLInboundDialector_TranslateIntoFilterOperatorSpec(t *testing.T) {
	dialector := &r3sql.SQLInboundDialector{}

	tests := []struct {
		name        string
		input       r3.DialectValue
		expected    r3.FilterOperatorSpec
		expectError bool
	}{
		{
			name:     "SQLClauseOperator value",
			input:    r3sql.SQLClauseOperatorEq,
			expected: r3.OperatorEq,
		},
		{
			name:     "SQLClauseOperator pointer",
			input:    func() *r3sql.SQLClauseOperator { op := r3sql.SQLClauseOperatorGt; return &op }(),
			expected: r3.OperatorGt,
		},
		{
			name:     "string value",
			input:    "LIKE",
			expected: r3.OperatorLike,
		},
		{
			name:     "all supported operators",
			input:    r3sql.SQLClauseOperatorNotIn,
			expected: r3.OperatorNotIn,
		},
		{
			name:        "unsupported operator",
			input:       r3sql.SQLClauseOperator("CUSTOM_OP"),
			expectError: true,
		},
		{
			name:        "invalid type",
			input:       123,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoFilterOperatorSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected, *result)
			}
		})
	}
}

func TestSQLInboundDialector_TranslateIntoSortSpec(t *testing.T) {
	dialector := &r3sql.SQLInboundDialector{}

	tests := []struct {
		name        string
		input       r3.DialectValue
		expectError bool
		validate    func(t *testing.T, result *r3.SortSpec)
	}{
		{
			name:  "SQLSort value - simple ASC",
			input: r3sql.SQLSort("name ASC"),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "name", result.Column.String())
				assert.Equal(t, r3.SortDirectionAsc, result.Direction)
			},
		},
		{
			name:  "SQLSort value - simple DESC",
			input: r3sql.SQLSort("created_at DESC"),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "created_at", result.Column.String())
				assert.Equal(t, r3.SortDirectionDesc, result.Direction)
			},
		},
		{
			name:  "SQLSort pointer",
			input: func() *r3sql.SQLSort { sort := r3sql.SQLSort("priority DESC"); return &sort }(),
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "priority", result.Column.String())
				assert.Equal(t, r3.SortDirectionDesc, result.Direction)
			},
		},
		{
			name:  "string value",
			input: "score ASC",
			validate: func(t *testing.T, result *r3.SortSpec) {
				assert.Equal(t, "score", result.Column.String())
				assert.Equal(t, r3.SortDirectionAsc, result.Direction)
			},
		},
		{
			name:        "invalid sort format",
			input:       r3sql.SQLSort("invalid_sort_format"),
			expectError: true,
		},
		{
			name:        "invalid type",
			input:       123,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoSortSpec(tt.input)

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

func TestSQLInboundDialector_TranslateIntoFilterSpec(t *testing.T) {
	dialector := &r3sql.SQLInboundDialector{}

	tests := []struct {
		name        string
		input       r3.DialectValue
		expectError bool
	}{
		{
			name:        "SQLClause value - not fully implemented",
			input:       r3sql.SQLClause{Clause: "id = ?", Args: []any{123}},
			expectError: true, // Expected because SQL to FilterSpec conversion is not fully implemented
		},
		{
			name:        "SQLClause pointer - not fully implemented",
			input:       &r3sql.SQLClause{Clause: "name LIKE ?", Args: []any{"John%"}},
			expectError: true, // Expected because SQL to FilterSpec conversion is not fully implemented
		},
		{
			name:        "invalid type",
			input:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoFilterSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

// func TestSqlToR3Operator(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		input       r3sql.SQLClauseOperator
// 		expected    r3.FilterOperatorSpec
// 		expectError bool
// 	}{
// 		{
// 			name:     "equals operator",
// 			input:    r3sql.SQLClauseOperatorEq,
// 			expected: r3.OperatorEq,
// 		},
// 		{
// 			name:     "not equals operator",
// 			input:    r3sql.SQLClauseOperatorNe,
// 			expected: r3.OperatorNe,
// 		},
// 		{
// 			name:     "greater than operator",
// 			input:    r3sql.SQLClauseOperatorGt,
// 			expected: r3.OperatorGt,
// 		},
// 		{
// 			name:     "greater than or equal operator",
// 			input:    r3sql.SQLClauseOperatorGte,
// 			expected: r3.OperatorGte,
// 		},
// 		{
// 			name:     "less than operator",
// 			input:    r3sql.SQLClauseOperatorLt,
// 			expected: r3.OperatorLt,
// 		},
// 		{
// 			name:     "less than or equal operator",
// 			input:    r3sql.SQLClauseOperatorLte,
// 			expected: r3.OperatorLte,
// 		},
// 		{
// 			name:     "like operator",
// 			input:    r3sql.SQLClauseOperatorLike,
// 			expected: r3.OperatorLike,
// 		},
// 		{
// 			name:     "not like operator",
// 			input:    r3sql.SQLClauseOperatorNotLike,
// 			expected: r3.OperatorNotLike,
// 		},
// 		{
// 			name:     "ilike operator",
// 			input:    r3sql.SQLClauseOperatorILike,
// 			expected: r3.OperatorILike,
// 		},
// 		{
// 			name:     "in operator",
// 			input:    r3sql.SQLClauseOperatorIn,
// 			expected: r3.OperatorIn,
// 		},
// 		{
// 			name:     "not in operator",
// 			input:    r3sql.SQLClauseOperatorNotIn,
// 			expected: r3.OperatorNotIn,
// 		},
// 		{
// 			name:        "unsupported operator",
// 			input:       r3sql.SQLClauseOperator("CUSTOM"),
// 			expected:    r3.OperatorUnspecified,
// 			expectError: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := sqlToR3Operator(tt.input)

// 			if tt.expectError {
// 				require.Error(t, err)
// 				assert.Equal(t, r3.OperatorUnspecified, result)
// 			} else {
// 				require.NoError(t, err)
// 				assert.Equal(t, tt.expected, result)
// 			}
// 		})
// 	}
// }

// func TestExtractJoinFromField(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		input    r3sql.SQLColumn
// 		expected []r3sql.SQLColumn
// 	}{
// 		{
// 			name:     "simple field - no join",
// 			input:    r3sql.SQLColumn("id"),
// 			expected: nil,
// 		},
// 		{
// 			name:     "field with join",
// 			input:    r3sql.SQLColumn("user.name"),
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user")},
// 		},
// 		{
// 			name:     "nested field with join",
// 			input:    r3sql.SQLColumn("orders.items.product.name"),
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("orders")},
// 		},
// 		{
// 			name:     "empty field",
// 			input:    r3sql.SQLColumn(""),
// 			expected: nil,
// 		},
// 		{
// 			name:     "field starting with dot",
// 			input:    r3sql.SQLColumn(".name"),
// 			expected: nil,
// 		},
// 		{
// 			name:     "field with only dots",
// 			input:    r3sql.SQLColumn("..."),
// 			expected: nil,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := extractJoinFromField(tt.input)
// 			if tt.expected == nil {
// 				assert.Nil(t, result)
// 			} else {
// 				require.NotNil(t, result)
// 				assert.Equal(t, tt.expected, result)
// 			}
// 		})
// 	}
// }

func TestSQLDialector_Integration(t *testing.T) {
	// Test realistic scenarios combining multiple features
	dialector := &r3sql.SQLDialector{}

	tests := []struct {
		name     string
		scenario string
		input    *r3.FilterSpec
		validate func(t *testing.T, result r3.DialectValue)
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
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "(email = ? AND status = ? AND last_login IS NOT NULL)", clause.Clause)
				require.Len(t, clause.Args, 2)
				assert.Equal(t, "user@example.com", clause.Args[0])
				assert.Equal(t, "active", clause.Args[1])
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
			validate: func(t *testing.T, result r3.DialectValue) {
				clause, ok := result.(r3sql.SQLClause)
				require.True(t, ok)
				assert.Equal(t, "(category.name IN ? AND (price < ? OR discount.active = ?))", clause.Clause)
				require.Len(t, clause.Args, 3)
				assert.Equal(t, []string{"electronics", "computers"}, clause.Args[0])
				assert.Equal(t, 1000, clause.Args[1])
				assert.Equal(t, true, clause.Args[2])

				// Should extract joins from category.name and discount.active
				expectedJoins := []r3sql.SQLColumn{r3sql.SQLColumn("category"), r3sql.SQLColumn("discount")}
				assert.ElementsMatch(t, expectedJoins, clause.Joins)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateFilterSpec(tt.input)
			require.NoError(t, err, "Scenario: %s", tt.scenario)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func BenchmarkSQLDialector_TranslateFilterSpec(b *testing.B) {
	dialector := &r3sql.SQLDialector{}

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
	for range b.N {
		_, err := dialector.TranslateFilterSpec(filter)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestParseSQLSortString(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *r3.SortSpec
		expectError bool
	}{
		{
			name:  "simple ascending sort",
			input: "name ASC",
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("name"); return &f }(),
				Direction:     r3.SortDirectionAsc,
				NullsPosition: r3.NullsPositionNotSpecified,
			},
		},
		{
			name:  "descending sort",
			input: "created_at DESC",
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("created_at"); return &f }(),
				Direction:     r3.SortDirectionDesc,
				NullsPosition: r3.NullsPositionNotSpecified,
			},
		},
		{
			name:  "field only (defaults to unspecified)",
			input: "id",
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("id"); return &f }(),
				Direction:     r3.SortDirectionUnspecified,
				NullsPosition: r3.NullsPositionNotSpecified,
			},
		},
		{
			name:  "sort with nulls first",
			input: "status DESC NULLS FIRST",
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("status"); return &f }(),
				Direction:     r3.SortDirectionDesc,
				NullsPosition: r3.NullsPositionFirst,
			},
		},
		{
			name:  "sort with nulls last",
			input: "updated_at ASC NULLS LAST",
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("updated_at"); return &f }(),
				Direction:     r3.SortDirectionAsc,
				NullsPosition: r3.NullsPositionLast,
			},
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "whitespace only",
			input:       "   ",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialector := &r3sql.SQLInboundDialector{}
			result, err := dialector.TranslateIntoSortSpec(r3sql.SQLSort(tt.input))

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Column.String(), result.Column.String())
				assert.Equal(t, tt.expected.Direction, result.Direction)
				assert.Equal(t, tt.expected.NullsPosition, result.NullsPosition)
			}
		})
	}
}
