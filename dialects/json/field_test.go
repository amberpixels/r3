package r3json_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	r3json "github.com/amberpixels/r3/dialects/json"
)

func TestJSONField_String(t *testing.T) {
	tests := []struct {
		name     string
		field    r3json.JSONField
		expected string
	}{
		{
			name:     "simple field name",
			field:    r3json.JSONField("id"),
			expected: "id",
		},
		{
			name:     "nested field name",
			field:    r3json.JSONField("user.profile.name"),
			expected: "user.profile.name",
		},
		{
			name:     "empty field name",
			field:    r3json.JSONField(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.field.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONFields_String(t *testing.T) {
	tests := []struct {
		name     string
		fields   r3json.JSONFields
		expected string
	}{
		{
			name:     "single field",
			fields:   r3json.JSONFields{r3json.JSONField("id")},
			expected: "id",
		},
		{
			name:     "multiple fields",
			fields:   r3json.JSONFields{r3json.JSONField("id"), r3json.JSONField("name"), r3json.JSONField("email")},
			expected: "id,name,email",
		},
		{
			name:     "empty fields",
			fields:   r3json.JSONFields{},
			expected: "",
		},
		{
			name:     "fields with empty names",
			fields:   r3json.JSONFields{r3json.JSONField("id"), r3json.JSONField(""), r3json.JSONField("name")},
			expected: "id,,name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fields.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONField_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    r3json.JSONField
		expectError bool
	}{
		{
			name:        "valid JSON string",
			input:       `"user_id"`,
			expected:    r3json.JSONField("user_id"),
			expectError: false,
		},
		{
			name:        "empty JSON string",
			input:       `""`,
			expected:    r3json.JSONField(""),
			expectError: false,
		},
		{
			name:        "invalid JSON",
			input:       `invalid`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var field r3json.JSONField
			err := json.Unmarshal([]byte(tt.input), &field)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, field)
			}
		})
	}
}

func TestJSONField_UnmarshalText(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected r3json.JSONField
	}{
		{
			name:     "regular text",
			input:    []byte("field_name"),
			expected: r3json.JSONField("field_name"),
		},
		{
			name:     "empty text",
			input:    []byte(""),
			expected: r3json.JSONField(""),
		},
		{
			name:     "text with special characters",
			input:    []byte("user.profile.email"),
			expected: r3json.JSONField("user.profile.email"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var field r3json.JSONField
			err := field.UnmarshalText(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, field)
		})
	}
}

func TestJSONField_MarshalText(t *testing.T) {
	tests := []struct {
		name     string
		field    r3json.JSONField
		expected []byte
	}{
		{
			name:     "regular field",
			field:    r3json.JSONField("field_name"),
			expected: []byte("field_name"),
		},
		{
			name:     "empty field",
			field:    r3json.JSONField(""),
			expected: []byte(""),
		},
		{
			name:     "field with special characters",
			field:    r3json.JSONField("user.profile.email"),
			expected: []byte("user.profile.email"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.field.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONField_ToFieldSpec(t *testing.T) {
	tests := []struct {
		name     string
		field    r3json.JSONField
		expected string
	}{
		{
			name:     "simple field",
			field:    r3json.JSONField("id"),
			expected: "id",
		},
		{
			name:     "nested field",
			field:    r3json.JSONField("user.profile.name"),
			expected: "user.profile.name",
		},
		{
			name:     "empty field",
			field:    r3json.JSONField(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.field.ToFieldSpec()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.String())
		})
	}
}

func TestJSONFields_ToFieldSpecs(t *testing.T) {
	tests := []struct {
		name     string
		fields   r3json.JSONFields
		expected []string
	}{
		{
			name:     "single field",
			fields:   r3json.JSONFields{r3json.JSONField("id")},
			expected: []string{"id"},
		},
		{
			name:     "multiple fields",
			fields:   r3json.JSONFields{r3json.JSONField("id"), r3json.JSONField("name"), r3json.JSONField("email")},
			expected: []string{"id", "name", "email"},
		},
		{
			name:     "empty fields",
			fields:   r3json.JSONFields{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.fields.ToFieldSpecs()
			require.NoError(t, err)

			assert.Len(t, result, len(tt.expected))
			for i, expectedField := range tt.expected {
				assert.Equal(t, expectedField, result[i].String())
			}
		})
	}
}

func TestJSONField_JSONRoundTrip(t *testing.T) {
	field := r3json.JSONField("test.field.name")

	// Marshal to JSON
	jsonData, err := json.Marshal(field)
	require.NoError(t, err)

	// Unmarshal from JSON
	var unmarshaled r3json.JSONField
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	// Should be equal
	assert.Equal(t, field, unmarshaled)
}

func TestJSONField_TextRoundTrip(t *testing.T) {
	field := r3json.JSONField("test.field.name")

	// Marshal to text
	textData, err := field.MarshalText()
	require.NoError(t, err)

	// Unmarshal from text
	var unmarshaled r3json.JSONField
	err = unmarshaled.UnmarshalText(textData)
	require.NoError(t, err)

	// Should be equal
	assert.Equal(t, field, unmarshaled)
}
