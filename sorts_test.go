package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
)

func TestSortSpec_DialectString(t *testing.T) {
	tests := []struct {
		name     string
		sort     *r3.SortSpec
		expected string
	}{
		{
			name:     "simple ascending sort",
			sort:     r3.NewSortAscSpec(r3.NewFieldSpec("name")),
			expected: "name ASC",
		},
		{
			name:     "descending sort",
			sort:     r3.NewSortDescSpec(r3.NewFieldSpec("created_at")),
			expected: "created_at DESC",
		},
		{
			name:     "sort with nulls first",
			sort:     r3.NewSortSpec(r3.NewFieldSpec("updated_at"), r3.SortDirectionAsc, r3.NullsPositionFirst),
			expected: "updated_at ASC NULLS FIRST",
		},
		{
			name:     "sort with nulls last",
			sort:     r3.NewSortSpec(r3.NewFieldSpec("status"), r3.SortDirectionDesc, r3.NullsPositionLast),
			expected: "status DESC NULLS LAST",
		},
		{
			name:     "unspecified direction defaults to DESC",
			sort:     r3.NewSortSpec(r3.NewFieldSpec("id"), r3.SortDirectionUnspecified),
			expected: "id DESC",
		},
	}

	sqlDialector := &r3sql.SQLDialector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialectValue, err := tt.sort.ToDialect(sqlDialector)
			if err != nil {
				t.Errorf("ToDialect() error = %v", err)
				return
			}

			sqlSort, ok := dialectValue.(r3sql.SQLSort)
			if !ok {
				t.Errorf("ToDialect() returned %T, expected r3sql.SQLSort", dialectValue)
				return
			}

			result := sqlSort.String()
			if result != tt.expected {
				t.Errorf("ToDialect().String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewSorts(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "single field ascending",
			input:    []string{"name ASC"},
			expected: []string{"name ASC"},
		},
		{
			name:     "multiple fields",
			input:    []string{"name ASC", "created_at DESC"},
			expected: []string{"name ASC", "created_at DESC"},
		},
		{
			name:     "field only (defaults to DESC)",
			input:    []string{"id"},
			expected: []string{"id DESC"},
		},
		{
			name:     "with nulls position",
			input:    []string{"status DESC NULLS LAST"},
			expected: []string{"status DESC NULLS LAST"},
		},
		{
			name:     "comma separated in single string",
			input:    []string{"name ASC, created_at DESC"},
			expected: []string{"name ASC", "created_at DESC"},
		},
	}

	sqlDialector := &r3sql.SQLDialector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorts, err := r3.NewSorts(tt.input...)
			if err != nil {
				t.Errorf("NewSorts() error = %v", err)
				return
			}

			if len(sorts) != len(tt.expected) {
				t.Errorf("NewSorts() length = %v, want %v", len(sorts), len(tt.expected))
				return
			}

			for i, sort := range sorts {
				dialectValue, err := sort.ToDialect(sqlDialector)
				if err != nil {
					t.Errorf("sorts[%d].ToDialect() error = %v", i, err)
					return
				}

				sqlSort, ok := dialectValue.(r3sql.SQLSort)
				if !ok {
					t.Errorf("sorts[%d].ToDialect() returned %T, expected r3sql.SQLSort", i, dialectValue)
					return
				}

				result := sqlSort.String()
				if result != tt.expected[i] {
					t.Errorf("sorts[%d].ToDialect().String() = %v, want %v", i, result, tt.expected[i])
				}
			}
		})
	}
}
