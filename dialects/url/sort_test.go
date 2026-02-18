package r3url_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSort_ColonDir(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []struct {
			field     string
			direction r3.SortDirection
		}
	}{
		{
			name:  "single ascending",
			input: "name:asc",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"name", r3.SortDirectionAsc},
			},
		},
		{
			name:  "single descending",
			input: "age:desc",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"age", r3.SortDirectionDesc},
			},
		},
		{
			name:  "multiple sorts",
			input: "name:asc,age:desc,id:asc",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"name", r3.SortDirectionAsc},
				{"age", r3.SortDirectionDesc},
				{"id", r3.SortDirectionAsc},
			},
		},
		{
			name:  "field only (defaults to asc)",
			input: "name",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"name", r3.SortDirectionAsc},
			},
		},
		{
			name:  "with spaces",
			input: " name : asc , age : desc ",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"name", r3.SortDirectionAsc},
				{"age", r3.SortDirectionDesc},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3url.ParseSort(tt.input, r3url.SortFormatColonDir)
			require.NoError(t, err)
			require.Len(t, result, len(tt.expected))

			for i, exp := range tt.expected {
				assert.Equal(t, exp.field, result[i].Column.String())
				assert.Equal(t, exp.direction, result[i].Direction)
			}
		})
	}
}

func TestParseSort_ColonDir_WithNulls(t *testing.T) {
	result, err := r3url.ParseSort("name:asc:first,age:desc:last", r3url.SortFormatColonDir)
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, "name", result[0].Column.String())
	assert.Equal(t, r3.SortDirectionAsc, result[0].Direction)
	assert.Equal(t, r3.NullsPositionFirst, result[0].NullsPosition)

	assert.Equal(t, "age", result[1].Column.String())
	assert.Equal(t, r3.SortDirectionDesc, result[1].Direction)
	assert.Equal(t, r3.NullsPositionLast, result[1].NullsPosition)
}

func TestParseSort_SignPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []struct {
			field     string
			direction r3.SortDirection
		}
	}{
		{
			name:  "ascending (no prefix)",
			input: "name",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"name", r3.SortDirectionAsc},
			},
		},
		{
			name:  "descending (- prefix)",
			input: "-age",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"age", r3.SortDirectionDesc},
			},
		},
		{
			name:  "multiple mixed",
			input: "name,-age,+id",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"name", r3.SortDirectionAsc},
				{"age", r3.SortDirectionDesc},
				{"id", r3.SortDirectionAsc},
			},
		},
		{
			name:  "with spaces",
			input: " name , -age ",
			expected: []struct {
				field     string
				direction r3.SortDirection
			}{
				{"name", r3.SortDirectionAsc},
				{"age", r3.SortDirectionDesc},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3url.ParseSort(tt.input, r3url.SortFormatSignPrefix)
			require.NoError(t, err)
			require.Len(t, result, len(tt.expected))

			for i, exp := range tt.expected {
				assert.Equal(t, exp.field, result[i].Column.String())
				assert.Equal(t, exp.direction, result[i].Direction)
			}
		})
	}
}

func TestParseSort_JSON(t *testing.T) {
	input := `[{"field":"name","direction":"asc"},{"field":"age","direction":"desc","nulls_position":"last"}]`
	result, err := r3url.ParseSort(input, r3url.SortFormatJSON)
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, "name", result[0].Column.String())
	assert.Equal(t, r3.SortDirectionAsc, result[0].Direction)

	assert.Equal(t, "age", result[1].Column.String())
	assert.Equal(t, r3.SortDirectionDesc, result[1].Direction)
	assert.Equal(t, r3.NullsPositionLast, result[1].NullsPosition)
}

func TestParseSort_Empty(t *testing.T) {
	for _, format := range []r3url.SortFormat{r3url.SortFormatColonDir, r3url.SortFormatSignPrefix, r3url.SortFormatJSON} {
		result, err := r3url.ParseSort("", format)
		require.NoError(t, err)
		assert.Nil(t, result)
	}
}

func TestFormatSort_ColonDir(t *testing.T) {
	sorts := r3.Sorts{
		&r3.SortSpec{Column: r3.NewFieldSpec("name"), Direction: r3.SortDirectionAsc},
		&r3.SortSpec{Column: r3.NewFieldSpec("age"), Direction: r3.SortDirectionDesc},
	}

	result, err := r3url.FormatSort(sorts, r3url.SortFormatColonDir)
	require.NoError(t, err)
	assert.Equal(t, "name:asc,age:desc", result)
}

func TestFormatSort_ColonDir_WithNulls(t *testing.T) {
	sorts := r3.Sorts{
		&r3.SortSpec{
			Column:        r3.NewFieldSpec("name"),
			Direction:     r3.SortDirectionAsc,
			NullsPosition: r3.NullsPositionFirst,
		},
	}

	result, err := r3url.FormatSort(sorts, r3url.SortFormatColonDir)
	require.NoError(t, err)
	assert.Equal(t, "name:asc:first", result)
}

func TestFormatSort_SignPrefix(t *testing.T) {
	sorts := r3.Sorts{
		&r3.SortSpec{Column: r3.NewFieldSpec("name"), Direction: r3.SortDirectionAsc},
		&r3.SortSpec{Column: r3.NewFieldSpec("age"), Direction: r3.SortDirectionDesc},
	}

	result, err := r3url.FormatSort(sorts, r3url.SortFormatSignPrefix)
	require.NoError(t, err)
	assert.Equal(t, "name,-age", result)
}

func TestSortRoundTrip_ColonDir(t *testing.T) {
	original := "name:asc,age:desc"
	sorts, err := r3url.ParseSort(original, r3url.SortFormatColonDir)
	require.NoError(t, err)

	formatted, err := r3url.FormatSort(sorts, r3url.SortFormatColonDir)
	require.NoError(t, err)
	assert.Equal(t, original, formatted)
}

func TestSortRoundTrip_SignPrefix(t *testing.T) {
	original := "name,-age"
	sorts, err := r3url.ParseSort(original, r3url.SortFormatSignPrefix)
	require.NoError(t, err)

	formatted, err := r3url.FormatSort(sorts, r3url.SortFormatSignPrefix)
	require.NoError(t, err)
	assert.Equal(t, original, formatted)
}
