package r3sql_test

import (
	"testing"

	r3sql "github.com/amberpixels/r3/dialects/sql"

	"github.com/stretchr/testify/assert"
)

func TestSQLClause_Joins(t *testing.T) {
	tests := []struct {
		name     string
		clause   r3sql.SQLClause
		expected []r3sql.SQLColumn
	}{
		{
			name: "no joins",
			clause: r3sql.SQLClause{
				Clause: "id = ?",
				Args:   []any{123},
				Joins:  []r3sql.SQLColumn{},
			},
			expected: []r3sql.SQLColumn{},
		},
		{
			name: "single join",
			clause: r3sql.SQLClause{
				Clause: "user.name = ?",
				Args:   []any{"John"},
				Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
			},
			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user")},
		},
		{
			name: "multiple joins",
			clause: r3sql.SQLClause{
				Clause: "user.profile.name = ?",
				Args:   []any{"John"},
				Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("profile")},
			},
			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("profile")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.clause.Joins)
		})
	}
}

func TestSQLClauses_Joins(t *testing.T) {
	tests := []struct {
		name     string
		clauses  r3sql.SQLClauses
		expected []r3sql.SQLColumn
	}{
		{
			name:     "empty clauses",
			clauses:  r3sql.SQLClauses{},
			expected: []r3sql.SQLColumn{},
		},
		{
			name: "single clause with no joins",
			clauses: r3sql.SQLClauses{
				{
					Clause: "id = ?",
					Args:   []any{123},
					Joins:  []r3sql.SQLColumn{},
				},
			},
			expected: []r3sql.SQLColumn{},
		},
		{
			name: "single clause with joins",
			clauses: r3sql.SQLClauses{
				{
					Clause: "user.name = ?",
					Args:   []any{"John"},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
				},
			},
			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user")},
		},
		{
			name: "multiple clauses with different joins",
			clauses: r3sql.SQLClauses{
				{
					Clause: "user.name = ?",
					Args:   []any{"John"},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
				},
				{
					Clause: "profile.age >= ?",
					Args:   []any{18},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("profile")},
				},
				{
					Clause: "address.city = ?",
					Args:   []any{"NYC"},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("address")},
				},
			},
			expected: []r3sql.SQLColumn{
				r3sql.SQLColumn("user"),
				r3sql.SQLColumn("profile"),
				r3sql.SQLColumn("address"),
			},
		},
		{
			name: "multiple clauses with overlapping joins",
			clauses: r3sql.SQLClauses{
				{
					Clause: "user.name = ?",
					Args:   []any{"John"},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
				},
				{
					Clause: "user.email = ?",
					Args:   []any{"john@example.com"},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
				},
				{
					Clause: "profile.age >= ?",
					Args:   []any{18},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("profile")},
				},
			},
			expected: []r3sql.SQLColumn{
				r3sql.SQLColumn("user"),
				r3sql.SQLColumn("profile"),
			}, // Note: deduplicated by quick.Append
		},
		{
			name: "multiple clauses with complex joins",
			clauses: r3sql.SQLClauses{
				{
					Clause: "orders.items.product.name LIKE ?",
					Args:   []any{"laptop%"},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("orders"), r3sql.SQLColumn("items")},
				},
				{
					Clause: "user.profile.status = ?",
					Args:   []any{"active"},
					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("profile")},
				},
			},
			expected: []r3sql.SQLColumn{
				r3sql.SQLColumn("orders"),
				r3sql.SQLColumn("items"),
				r3sql.SQLColumn("user"),
				r3sql.SQLColumn("profile"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.clauses.Joins()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// func TestExtractJoinFromField_Detailed(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		field    r3sql.SQLColumn
// 		expected []r3sql.SQLColumn
// 	}{
// 		{
// 			name:     "simple field with no join",
// 			field:    r3sql.SQLColumn("id"),
// 			expected: nil,
// 		},
// 		{
// 			name:     "field with single join",
// 			field:    r3sql.SQLColumn("user.name"),
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user")},
// 		},
// 		{
// 			name:     "field with nested joins",
// 			field:    r3sql.SQLColumn("user.profile.address.city"),
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user")}, // Only extracts first level
// 		},
// 		{
// 			name:     "empty field",
// 			field:    r3sql.SQLColumn(""),
// 			expected: nil,
// 		},
// 		{
// 			name:     "field starting with dot",
// 			field:    r3sql.SQLColumn(".field"),
// 			expected: nil, // Empty prefix before dot
// 		},
// 		{
// 			name:     "field ending with dot",
// 			field:    r3sql.SQLColumn("table."),
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("table")},
// 		},
// 		{
// 			name:     "field with multiple dots",
// 			field:    r3sql.SQLColumn("a.b.c.d"),
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("a")}, // Only first level
// 		},
// 		{
// 			name:     "field with spaces (unusual but valid)",
// 			field:    r3sql.SQLColumn("my table.my field"),
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("my table")},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := extractJoinFromField(tt.field)
// 			if tt.expected == nil {
// 				assert.Nil(t, result)
// 			} else {
// 				require.NotNil(t, result)
// 				assert.Equal(t, tt.expected, result)
// 			}
// 		})
// 	}
// }

func TestSQLColumn_String(t *testing.T) {
	tests := []struct {
		name     string
		column   r3sql.SQLColumn
		expected string
	}{
		{
			name:     "simple column",
			column:   r3sql.SQLColumn("id"),
			expected: "id",
		},
		{
			name:     "column with table",
			column:   r3sql.SQLColumn("users.id"),
			expected: "users.id",
		},
		{
			name:     "complex column path",
			column:   r3sql.SQLColumn("orders.items.product.name"),
			expected: "orders.items.product.name",
		},
		{
			name:     "empty column",
			column:   r3sql.SQLColumn(""),
			expected: "",
		},
		{
			name:     "column with underscores",
			column:   r3sql.SQLColumn("user_profiles.full_name"),
			expected: "user_profiles.full_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.column.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSQLClause_Structure(t *testing.T) {
	// Test that SQLClause has the expected structure and behavior
	tests := []struct {
		name   string
		clause r3sql.SQLClause
	}{
		{
			name: "basic clause",
			clause: r3sql.SQLClause{
				Clause: "name = ?",
				Args:   []any{"John"},
				Joins:  []r3sql.SQLColumn{},
			},
		},
		{
			name: "clause with multiple args",
			clause: r3sql.SQLClause{
				Clause: "age BETWEEN ? AND ?",
				Args:   []any{18, 65},
				Joins:  []r3sql.SQLColumn{},
			},
		},
		{
			name: "clause with joins",
			clause: r3sql.SQLClause{
				Clause: "user.profile.status = ?",
				Args:   []any{"active"},
				Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("profile")},
			},
		},
		{
			name: "complex clause",
			clause: r3sql.SQLClause{
				Clause: "(user.name LIKE ? AND profile.age >= ?) OR vip.status = ?",
				Args:   []any{"John%", 21, true},
				Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("profile"), r3sql.SQLColumn("vip")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the clause maintains its structure
			assert.NotEmpty(t, tt.clause.Clause)
			assert.NotNil(t, tt.clause.Args)
			assert.NotNil(t, tt.clause.Joins)

			// Test that Args and placeholders match (basic validation)
			placeholderCount := 0
			for _, char := range tt.clause.Clause {
				if char == '?' {
					placeholderCount++
				}
			}
			assert.Len(t, tt.clause.Args, placeholderCount, "Number of placeholders should match number of args")
		})
	}
}

func TestSQLClause_Integration(t *testing.T) {
	// Test realistic SQL clause scenarios
	tests := []struct {
		name        string
		description string
		clause      r3sql.SQLClause
		validate    func(t *testing.T, clause r3sql.SQLClause)
	}{
		{
			name:        "user authentication query",
			description: "Typical user login validation",
			clause: r3sql.SQLClause{
				Clause: "(email = ? AND password_hash = ?) AND status = ? AND deleted_at IS NULL",
				Args:   []any{"user@example.com", "hashed_password", "active"},
				Joins:  []r3sql.SQLColumn{},
			},
			validate: func(t *testing.T, clause r3sql.SQLClause) {
				assert.Contains(t, clause.Clause, "email = ?")
				assert.Contains(t, clause.Clause, "password_hash = ?")
				assert.Contains(t, clause.Clause, "status = ?")
				assert.Contains(t, clause.Clause, "deleted_at IS NULL")
				assert.Len(t, clause.Args, 3)
				assert.Empty(t, clause.Joins)
			},
		},
		{
			name:        "product search with joins",
			description: "Product search across multiple related tables",
			clause: r3sql.SQLClause{
				Clause: "category.name IN ? AND (price < ? OR discount.percentage > ?)",
				Args:   []any{[]string{"electronics", "computers"}, 1000, 10},
				Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("category"), r3sql.SQLColumn("discount")},
			},
			validate: func(t *testing.T, clause r3sql.SQLClause) {
				assert.Contains(t, clause.Clause, "category.name IN ?")
				assert.Contains(t, clause.Clause, "price < ?")
				assert.Contains(t, clause.Clause, "discount.percentage > ?")
				assert.Len(t, clause.Args, 3)
				assert.Contains(t, clause.Joins, r3sql.SQLColumn("category"))
				assert.Contains(t, clause.Joins, r3sql.SQLColumn("discount"))
			},
		},
		{
			name:        "audit log filtering",
			description: "Complex audit log query with time ranges and exclusions",
			clause: r3sql.SQLClause{
				Clause: "created_at BETWEEN ? AND ? AND user.role != ? AND action.type NOT IN ?",
				Args:   []any{"2023-01-01", "2023-12-31", "system", []string{"login", "logout"}},
				Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("action")},
			},
			validate: func(t *testing.T, clause r3sql.SQLClause) {
				assert.Contains(t, clause.Clause, "created_at BETWEEN")
				assert.Contains(t, clause.Clause, "user.role != ?")
				assert.Contains(t, clause.Clause, "action.type NOT IN ?")
				assert.Len(t, clause.Args, 4)
				expectedJoins := []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("action")}
				assert.ElementsMatch(t, expectedJoins, clause.Joins)
			},
		},
		{
			name:        "nested conditions",
			description: "Complex nested AND/OR conditions",
			clause: r3sql.SQLClause{
				Clause: "((user.status = ? AND subscription.active = ?) OR vip.level >= ?) AND deleted_at IS NULL",
				Args:   []any{"active", true, 3},
				Joins: []r3sql.SQLColumn{
					r3sql.SQLColumn("user"),
					r3sql.SQLColumn("subscription"),
					r3sql.SQLColumn("vip"),
				},
			},
			validate: func(t *testing.T, clause r3sql.SQLClause) {
				assert.Contains(t, clause.Clause, "user.status = ?")
				assert.Contains(t, clause.Clause, "subscription.active = ?")
				assert.Contains(t, clause.Clause, "vip.level >= ?")
				assert.Contains(t, clause.Clause, "deleted_at IS NULL")
				assert.Len(t, clause.Args, 3)
				assert.Len(t, clause.Joins, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.validate != nil {
				tt.validate(t, tt.clause)
			}
		})
	}
}

// func TestSQLClauses_CombinedJoins(t *testing.T) {
// 	// Test combining joins from multiple clauses in realistic scenarios
// 	tests := []struct {
// 		name        string
// 		description string
// 		clauses     r3sql.SQLClauses
// 		expected    []r3sql.SQLColumn
// 	}{
// 		{
// 			name:        "user profile query",
// 			description: "Query involving user, profile, and address tables",
// 			clauses: r3sql.SQLClauses{
// 				{
// 					Clause: "user.email = ?",
// 					Args:   []any{"user@example.com"},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
// 				},
// 				{
// 					Clause: "profile.status = ?",
// 					Args:   []any{"active"},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("profile")},
// 				},
// 				{
// 					Clause: "address.country = ?",
// 					Args:   []any{"USA"},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("address")},
// 				},
// 			},
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("profile"), r3sql.SQLColumn("address")},
// 		},
// 		{
// 			name:        "e-commerce order query",
// 			description: "Complex query involving orders, products, and customers",
// 			clauses: r3sql.SQLClauses{
// 				{
// 					Clause: "orders.status IN ?",
// 					Args:   []any{[]string{"shipped", "delivered"}},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("orders")},
// 				},
// 				{
// 					Clause: "products.category = ? AND products.price <= ?",
// 					Args:   []any{"electronics", 500},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("products")},
// 				},
// 				{
// 					Clause: "customers.tier = ? OR customers.vip = ?",
// 					Args:   []any{"premium", true},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("customers")},
// 				},
// 			},
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("orders"), r3sql.SQLColumn("products"), r3sql.SQLColumn("customers")},
// 		},
// 		{
// 			name:        "overlapping joins",
// 			description: "Multiple clauses referencing the same tables",
// 			clauses: r3sql.SQLClauses{
// 				{
// 					Clause: "user.name LIKE ?",
// 					Args:   []any{"John%"},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
// 				},
// 				{
// 					Clause: "user.age >= ?",
// 					Args:   []any{18},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
// 				},
// 				{
// 					Clause: "profile.verified = ?",
// 					Args:   []any{true},
// 					Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("profile")},
// 				},
// 			},
// 			expected: []r3sql.SQLColumn{r3sql.SQLColumn("user"), r3sql.SQLColumn("user"), r3sql.SQLColumn("profile")}, // Includes duplicates
// 		},
// 		{
// 			name:        "no joins scenario",
// 			description: "Queries that don't require any joins",
// 			clauses: r3sql.SQLClauses{
// 				{
// 					Clause: "id > ?",
// 					Args:   []any{100},
// 					Joins:  []r3sql.SQLColumn{},
// 				},
// 				{
// 					Clause: "created_at >= ?",
// 					Args:   []any{"2023-01-01"},
// 					Joins:  []r3sql.SQLColumn{},
// 				},
// 				{
// 					Clause: "status = ?",
// 					Args:   []any{"active"},
// 					Joins:  []r3sql.SQLColumn{},
// 				},
// 			},
// 			expected: []r3sql.SQLColumn{},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := tt.clauses.Joins()
// 			assert.Equal(t, tt.expected, result, tt.description)
// 		})
// 	}
// }

// func BenchmarkExtractJoinFromField(b *testing.B) {
// 	testFields := []r3sql.SQLColumn{
// 		r3sql.SQLColumn("id"),
// 		r3sql.SQLColumn("user.name"),
// 		r3sql.SQLColumn("orders.items.product.name"),
// 		r3sql.SQLColumn("deeply.nested.table.column.field"),
// 		r3sql.SQLColumn(""),
// 		r3sql.SQLColumn("simple_field"),
// 	}

// 	b.ResetTimer()
// 	for i := range b.N {
// 		field := testFields[i%len(testFields)]
// 		extractJoinFromField(field)
// 	}
// }

func BenchmarkSQLClause_Joins(b *testing.B) {
	clauses := r3sql.SQLClauses{
		{
			Clause: "user.name = ?",
			Args:   []any{"John"},
			Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("user")},
		},
		{
			Clause: "profile.age >= ?",
			Args:   []any{18},
			Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("profile")},
		},
		{
			Clause: "orders.status IN ?",
			Args:   []any{[]string{"active", "pending"}},
			Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("orders")},
		},
		{
			Clause: "address.city = ?",
			Args:   []any{"NYC"},
			Joins:  []r3sql.SQLColumn{r3sql.SQLColumn("address")},
		},
	}

	b.ResetTimer()
	for range b.N {
		clauses.Joins()
	}
}

func TestSQLClause_ArgumentValidation(t *testing.T) {
	// Test various argument types that might be passed to SQL clauses
	tests := []struct {
		name      string
		clause    string
		args      []any
		valid     bool
		expectLen int
	}{
		{
			name:      "string argument",
			clause:    "name = ?",
			args:      []any{"John Doe"},
			valid:     true,
			expectLen: 1,
		},
		{
			name:      "integer argument",
			clause:    "age >= ?",
			args:      []any{21},
			valid:     true,
			expectLen: 1,
		},
		{
			name:      "boolean argument",
			clause:    "active = ?",
			args:      []any{true},
			valid:     true,
			expectLen: 1,
		},
		{
			name:      "float argument",
			clause:    "price <= ?",
			args:      []any{99.99},
			valid:     true,
			expectLen: 1,
		},
		{
			name:      "slice argument for IN",
			clause:    "category IN ?",
			args:      []any{[]string{"electronics", "books"}},
			valid:     true,
			expectLen: 1,
		},
		{
			name:      "multiple arguments",
			clause:    "created_at BETWEEN ? AND ?",
			args:      []any{"2023-01-01", "2023-12-31"},
			valid:     true,
			expectLen: 2,
		},
		{
			name:      "nil argument",
			clause:    "deleted_at = ?",
			args:      []any{nil},
			valid:     true,
			expectLen: 1,
		},
		{
			name:      "empty args",
			clause:    "deleted_at IS NULL",
			args:      []any{},
			valid:     true,
			expectLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlClause := r3sql.SQLClause{
				Clause: tt.clause,
				Args:   tt.args,
			}

			assert.Len(t, sqlClause.Args, tt.expectLen)
			assert.NotNil(t, sqlClause.Args)

			// Count placeholders in clause
			placeholderCount := 0
			for _, char := range sqlClause.Clause {
				if char == '?' {
					placeholderCount++
				}
			}

			if tt.valid {
				assert.Len(t, sqlClause.Args, placeholderCount,
					"Placeholder count should match argument count for valid clauses")
			}
		})
	}
}
