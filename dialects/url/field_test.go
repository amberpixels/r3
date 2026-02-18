package r3url_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFields(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single field",
			input:    "id",
			expected: []string{"id"},
		},
		{
			name:     "multiple fields",
			input:    "id,name,status",
			expected: []string{"id", "name", "status"},
		},
		{
			name:     "fields with spaces",
			input:    " id , name , status ",
			expected: []string{"id", "name", "status"},
		},
		{
			name:     "dotted path fields",
			input:    "user.name,user.profile.email",
			expected: []string{"user.name", "user.profile.email"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: nil,
		},
		{
			name:     "trailing comma",
			input:    "id,name,",
			expected: []string{"id", "name"},
		},
		{
			name:     "leading comma",
			input:    ",id,name",
			expected: []string{"id", "name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3url.ParseFields(tt.input)
			require.NoError(t, err)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.Len(t, result, len(tt.expected))
				for i, exp := range tt.expected {
					assert.Equal(t, exp, result[i].String())
				}
			}
		})
	}
}

func TestFormatFields(t *testing.T) {
	tests := []struct {
		name     string
		input    r3.Fields
		expected string
	}{
		{
			name:     "multiple fields",
			input:    r3.Fields{r3.NewFieldSpec("id"), r3.NewFieldSpec("name"), r3.NewFieldSpec("status")},
			expected: "id,name,status",
		},
		{
			name:     "single field",
			input:    r3.Fields{r3.NewFieldSpec("id")},
			expected: "id",
		},
		{
			name:     "empty fields",
			input:    nil,
			expected: "",
		},
		{
			name:     "dotted path field",
			input:    r3.Fields{r3.NewFieldSpec("user.name")},
			expected: "user.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3url.FormatFields(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFieldsRoundTrip(t *testing.T) {
	original := "id,name,user.profile.email,status"
	fields, err := r3url.ParseFields(original)
	require.NoError(t, err)

	formatted := r3url.FormatFields(fields)
	assert.Equal(t, original, formatted)
}
