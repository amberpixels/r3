package r3sql_test

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
			expected: `"name" ASC`,
		},
		{
			name:     "descending sort",
			sort:     r3.NewSortDescSpec(r3.NewFieldSpec("created_at")),
			expected: `"created_at" DESC`,
		},
		{
			name:     "sort with nulls first",
			sort:     r3.NewSortSpec(r3.NewFieldSpec("updated_at"), r3.SortDirectionAsc, r3.NullsPositionFirst),
			expected: `"updated_at" ASC NULLS FIRST`,
		},
		{
			name:     "sort with nulls last",
			sort:     r3.NewSortSpec(r3.NewFieldSpec("status"), r3.SortDirectionDesc, r3.NullsPositionLast),
			expected: `"status" DESC NULLS LAST`,
		},
		{
			name:     "unspecified direction defaults to DESC",
			sort:     r3.NewSortSpec(r3.NewFieldSpec("id"), r3.SortDirectionUnspecified),
			expected: `"id" DESC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlSort, err := r3sql.SortToSQL(tt.sort)
			if err != nil {
				t.Errorf("SortToSQL() error = %v", err)
				return
			}

			result := sqlSort.String()
			if result != tt.expected {
				t.Errorf("SortToSQL().String() = %v, want %v", result, tt.expected)
			}
		})
	}
}
