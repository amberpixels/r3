package r3_test

import (
	"testing"

	"github.com/amberpixels/r3"
)

func TestSortSpec_DialectString(t *testing.T) {
	tests := []struct {
		name     string
		sort     *r3.SortSpec
		expected string
	}{
		{
			name: "simple ascending sort",
			sort: &r3.SortSpec{
				Column:    r3.FieldSpec("name"),
				Direction: r3.SortDirectionAsc,
			},
			expected: "name ASC",
		},
		{
			name: "descending sort",
			sort: &r3.SortSpec{
				Column:    r3.FieldSpec("created_at"),
				Direction: r3.SortDirectionDesc,
			},
			expected: "created_at DESC",
		},
		{
			name: "sort with nulls first",
			sort: &r3.SortSpec{
				Column:        r3.FieldSpec("updated_at"),
				Direction:     r3.SortDirectionAsc,
				NullsPosition: r3.NullsPositionFirst,
			},
			expected: "updated_at ASC NULLS FIRST",
		},
		{
			name: "sort with nulls last",
			sort: &r3.SortSpec{
				Column:        r3.FieldSpec("status"),
				Direction:     r3.SortDirectionDesc,
				NullsPosition: r3.NullsPositionLast,
			},
			expected: "status DESC NULLS LAST",
		},
		{
			name: "unspecified direction defaults to DESC",
			sort: &r3.SortSpec{
				Column:    r3.FieldSpec("id"),
				Direction: r3.SortDirectionUnspecified,
			},
			expected: "id DESC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.sort.DialectString()
			if result != tt.expected {
				t.Errorf("DialectString() = %v, want %v", result, tt.expected)
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
				result := sort.DialectString()
				if result != tt.expected[i] {
					t.Errorf("sorts[%d].DialectString() = %v, want %v", i, result, tt.expected[i])
				}
			}
		})
	}
}
