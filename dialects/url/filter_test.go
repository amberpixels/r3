package r3url_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
)

func TestFormatFilters_RelationFilterRejected(t *testing.T) {
	// A relationship ("has") filter can't be serialized into URL params; the
	// JSON-backed formatter must surface an error rather than drop it silently.
	_, err := r3url.FormatFilters(r3.Filters{r3.Has("Squads", r3.In("id", []int64{1, 3}))})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Squads")
}

func TestParseFilters(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(t *testing.T, result r3.Filters)
	}{
		{
			name:  "single filter",
			input: `[{"f":"status","op":"eq","v":"active"}]`,
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				assert.Equal(t, "status", result[0].Field.String())
				assert.Equal(t, r3.OperatorEq, result[0].Operator)
				assert.Equal(t, "active", result[0].Value)
			},
		},
		{
			name:  "multiple filters",
			input: `[{"f":"status","op":"eq","v":"active"},{"f":"age","op":"gte","v":18}]`,
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 2)
				assert.Equal(t, "status", result[0].Field.String())
				assert.Equal(t, "age", result[1].Field.String())
			},
		},
		{
			name:  "nested AND/OR",
			input: `[{"and":[{"f":"status","op":"eq","v":"active"},{"or":[{"f":"age","op":"gte","v":18},{"f":"role","op":"eq","v":"admin"}]}]}]`,
			validate: func(t *testing.T, result r3.Filters) {
				require.Len(t, result, 1)
				require.Len(t, result[0].And, 2)
				require.Len(t, result[0].And[1].Or, 2)
			},
		},
		{
			name:  "empty string",
			input: "",
			validate: func(t *testing.T, result r3.Filters) {
				assert.Nil(t, result)
			},
		},
		{
			name:        "invalid JSON",
			input:       `not-json`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3url.ParseFilters(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.True(t, r3url.IsErrDialectorFailure(err))
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestFormatFilters(t *testing.T) {
	filters := r3.Filters{
		&r3.FilterSpec{
			Field:    r3.NewFieldSpec("status"),
			Operator: r3.OperatorEq,
			Value:    "active",
		},
	}

	result, err := r3url.FormatFilters(filters)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	// Parse it back to verify round-trip
	parsed, err := r3url.ParseFilters(result)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, "status", parsed[0].Field.String())
	assert.Equal(t, r3.OperatorEq, parsed[0].Operator)
}

func TestFormatFilters_Empty(t *testing.T) {
	result, err := r3url.FormatFilters(nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}
