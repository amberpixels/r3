package r3json_test

import (
	"encoding/json"
	"testing"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONSort_String(t *testing.T) {
	tests := []struct {
		name     string
		sort     *r3json.JSONSort
		expected string
	}{
		{
			name: "simple ascending sort",
			sort: &r3json.JSONSort{
				Field:     r3json.JSONField("name"),
				Direction: r3json.JSONSortDirectionAsc,
			},
			expected: `{"field":"name","direction":"asc"}`,
		},
		{
			name: "descending sort with nulls last",
			sort: &r3json.JSONSort{
				Field:         r3json.JSONField("created_at"),
				Direction:     r3json.JSONSortDirectionDesc,
				NullsPosition: r3json.JSONNullsPositionLast,
			},
			expected: `{"field":"created_at","direction":"desc","nulls_position":"last"}`,
		},
		{
			name: "field only (no direction or nulls)",
			sort: &r3json.JSONSort{
				Field: r3json.JSONField("id"),
			},
			expected: `{"field":"id"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.sort.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONSort_ToSortSpec(t *testing.T) {
	tests := []struct {
		name     string
		sort     *r3json.JSONSort
		expected *r3.SortSpec
	}{
		{
			name: "ascending sort",
			sort: &r3json.JSONSort{
				Field:     r3json.JSONField("name"),
				Direction: r3json.JSONSortDirectionAsc,
			},
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("name"); return &f }(),
				Direction:     r3.SortDirectionAsc,
				NullsPosition: r3.NullsPositionNotSpecified,
			},
		},
		{
			name: "descending sort with nulls first",
			sort: &r3json.JSONSort{
				Field:         r3json.JSONField("updated_at"),
				Direction:     r3json.JSONSortDirectionDesc,
				NullsPosition: r3json.JSONNullsPositionFirst,
			},
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("updated_at"); return &f }(),
				Direction:     r3.SortDirectionDesc,
				NullsPosition: r3.NullsPositionFirst,
			},
		},
		{
			name: "unspecified direction and nulls",
			sort: &r3json.JSONSort{
				Field:         r3json.JSONField("status"),
				Direction:     r3json.JSONSortDirectionUnspecified,
				NullsPosition: r3json.JSONNullsPositionUnspecified,
			},
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("status"); return &f }(),
				Direction:     r3.SortDirectionUnspecified,
				NullsPosition: r3.NullsPositionNotSpecified,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.sort.ToSortSpec()
			require.NoError(t, err)
			assert.Equal(t, tt.expected.Column.String(), result.Column.String())
			assert.Equal(t, tt.expected.Direction, result.Direction)
			assert.Equal(t, tt.expected.NullsPosition, result.NullsPosition)
		})
	}
}

func TestJSONSortDirection_ToR3SortDirection(t *testing.T) {
	tests := []struct {
		name     string
		input    r3json.JSONSortDirection
		expected r3.SortDirection
	}{
		{
			name:     "ascending",
			input:    r3json.JSONSortDirectionAsc,
			expected: r3.SortDirectionAsc,
		},
		{
			name:     "descending",
			input:    r3json.JSONSortDirectionDesc,
			expected: r3.SortDirectionDesc,
		},
		{
			name:     "unspecified",
			input:    r3json.JSONSortDirectionUnspecified,
			expected: r3.SortDirectionUnspecified,
		},
		{
			name:     "invalid/unknown",
			input:    r3json.JSONSortDirection("invalid"),
			expected: r3.SortDirectionUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToR3SortDirection()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONNullsPosition_ToR3NullsPosition(t *testing.T) {
	tests := []struct {
		name     string
		input    r3json.JSONNullsPosition
		expected r3.SortNullsPosition
	}{
		{
			name:     "first",
			input:    r3json.JSONNullsPositionFirst,
			expected: r3.NullsPositionFirst,
		},
		{
			name:     "last",
			input:    r3json.JSONNullsPositionLast,
			expected: r3.NullsPositionLast,
		},
		{
			name:     "unspecified",
			input:    r3json.JSONNullsPositionUnspecified,
			expected: r3.NullsPositionNotSpecified,
		},
		{
			name:     "invalid/unknown",
			input:    r3json.JSONNullsPosition("invalid"),
			expected: r3.NullsPositionNotSpecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ToR3NullsPosition()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONSort_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		sort     *r3json.JSONSort
		expected string
	}{
		{
			name: "full sort specification",
			sort: &r3json.JSONSort{
				Field:         r3json.JSONField("name"),
				Direction:     r3json.JSONSortDirectionAsc,
				NullsPosition: r3json.JSONNullsPositionLast,
			},
			expected: `{"field":"name","direction":"asc","nulls_position":"last"}`,
		},
		{
			name: "minimal sort specification",
			sort: &r3json.JSONSort{
				Field: r3json.JSONField("id"),
			},
			expected: `{"field":"id"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.sort)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(jsonData))

			// Test unmarshaling
			var unmarshaledSort r3json.JSONSort
			err = json.Unmarshal(jsonData, &unmarshaledSort)
			require.NoError(t, err)
			assert.Equal(t, tt.sort.Field, unmarshaledSort.Field)
			assert.Equal(t, tt.sort.Direction, unmarshaledSort.Direction)
			assert.Equal(t, tt.sort.NullsPosition, unmarshaledSort.NullsPosition)
		})
	}
}

func TestJSONInboundDialector_TranslateIntoSortSpec(t *testing.T) {
	tests := []struct {
		name        string
		input       r3.DialectValue
		expected    *r3.SortSpec
		expectError bool
	}{
		{
			name: "valid JSONSort",
			input: &r3json.JSONSort{
				Field:     r3json.JSONField("name"),
				Direction: r3json.JSONSortDirectionAsc,
			},
			expected: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("name"); return &f }(),
				Direction:     r3.SortDirectionAsc,
				NullsPosition: r3.NullsPositionNotSpecified,
			},
		},
		{
			name:        "invalid input type",
			input:       "invalid",
			expectError: true,
		},
	}

	dialector := &r3json.JSONInboundDialector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoSortSpec(tt.input)

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

func TestJSONOutboundDialector_TranslateSortSpec(t *testing.T) {
	tests := []struct {
		name        string
		input       *r3.SortSpec
		expected    *r3json.JSONSort
		expectError bool
	}{
		{
			name: "valid SortSpec",
			input: &r3.SortSpec{
				Column:        func() *r3.FieldSpec { f := r3.FieldSpec("name"); return &f }(),
				Direction:     r3.SortDirectionDesc,
				NullsPosition: r3.NullsPositionFirst,
			},
			expected: &r3json.JSONSort{
				Field:         r3json.JSONField("name"),
				Direction:     r3json.JSONSortDirectionDesc,
				NullsPosition: r3json.JSONNullsPositionFirst,
			},
		},
		{
			name:        "nil SortSpec",
			input:       nil,
			expectError: true,
		},
	}

	dialector := &r3json.JSONOutboundDialector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateSortSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				jsonSort, ok := result.(*r3json.JSONSort)
				require.True(t, ok, "Result should be a JSONSort")
				assert.Equal(t, tt.expected.Field, jsonSort.Field)
				assert.Equal(t, tt.expected.Direction, jsonSort.Direction)
				assert.Equal(t, tt.expected.NullsPosition, jsonSort.NullsPosition)
			}
		})
	}
}
