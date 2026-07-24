package r3json_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
)

func TestJSONFilterOperator_String(t *testing.T) {
	tests := []struct {
		name     string
		operator r3json.JSONFilterOperator
		expected string
	}{
		{
			name:     "OperatorEq",
			operator: r3json.OperatorEq,
			expected: "eq",
		},
		{
			name:     "OperatorNe",
			operator: r3json.OperatorNe,
			expected: "ne",
		},
		{
			name:     "OperatorGt",
			operator: r3json.OperatorGt,
			expected: "gt",
		},
		{
			name:     "OperatorLte",
			operator: r3json.OperatorLte,
			expected: "lte",
		},
		{
			name:     "OperatorLike",
			operator: r3json.OperatorLike,
			expected: "like",
		},
		{
			name:     "OperatorUnspecified",
			operator: r3json.OperatorUnspecified,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.operator.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONFilterOperator_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    r3json.JSONFilterOperator
		expectError bool
	}{
		{
			name:        "valid eq operator",
			input:       `"eq"`,
			expected:    r3json.OperatorEq,
			expectError: false,
		},
		{
			name:        "valid gt operator",
			input:       `"gt"`,
			expected:    r3json.OperatorGt,
			expectError: false,
		},
		{
			name:        "valid symbol operator",
			input:       `">="`,
			expected:    r3json.OperatorGte,
			expectError: false,
		},
		{
			name:        "valid between operator",
			input:       `"between"`,
			expected:    r3json.OperatorBetween,
			expectError: false,
		},
		{
			name:        "invalid operator",
			input:       `"invalid"`,
			expectError: true,
		},
		{
			name:        "empty string operator",
			input:       `""`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var op r3json.JSONFilterOperator
			err := json.Unmarshal([]byte(tt.input), &op)

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, r3json.ErrInvalidFilterOperator)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, op)
			}
		})
	}
}

func TestJSONFilterOperator_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		operator r3json.JSONFilterOperator
		expected []byte
	}{
		{
			name:     "OperatorEq",
			operator: r3json.OperatorEq,
			expected: []byte(`"eq"`),
		},
		{
			name:     "OperatorGte",
			operator: r3json.OperatorGte,
			expected: []byte(`"gte"`),
		},
		{
			name:     "OperatorBetween",
			operator: r3json.OperatorBetween,
			expected: []byte(`"between"`),
		},
		{
			name:     "OperatorUnspecified",
			operator: r3json.OperatorUnspecified,
			expected: []byte(`""`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.operator.MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONFilterOperator_UnmarshalText(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expected    r3json.JSONFilterOperator
		expectError bool
	}{
		{
			name:        "valid eq operator",
			input:       []byte("eq"),
			expected:    r3json.OperatorEq,
			expectError: false,
		},
		{
			name:        "valid symbol ==",
			input:       []byte("=="),
			expected:    r3json.OperatorEq,
			expectError: false,
		},
		{
			name:        "valid symbol >",
			input:       []byte(">"),
			expected:    r3json.OperatorGt,
			expectError: false,
		},
		{
			name:        "valid like symbol",
			input:       []byte("*="),
			expected:    r3json.OperatorLike,
			expectError: false,
		},
		{
			name:        "valid in operator",
			input:       []byte("in"),
			expected:    r3json.OperatorIn,
			expectError: false,
		},
		{
			name:        "invalid operator",
			input:       []byte("invalid"),
			expectError: true,
		},
		{
			name:        "empty operator",
			input:       []byte(""),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var op r3json.JSONFilterOperator
			err := op.UnmarshalText(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.ErrorIs(t, err, r3json.ErrInvalidFilterOperator)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, op)
			}
		})
	}
}

func TestJSONFilterOperator_MarshalText(t *testing.T) {
	tests := []struct {
		name     string
		operator r3json.JSONFilterOperator
		expected []byte
	}{
		{
			name:     "OperatorEq",
			operator: r3json.OperatorEq,
			expected: []byte("eq"),
		},
		{
			name:     "OperatorNotLike",
			operator: r3json.OperatorNotLike,
			expected: []byte("notlike"),
		},
		{
			name:     "OperatorBetweenEx",
			operator: r3json.OperatorBetweenEx,
			expected: []byte("between_exc"),
		},
		{
			name:     "OperatorUnspecified",
			operator: r3json.OperatorUnspecified,
			expected: []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.operator.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONFilterOperator_ToFilterOperatorSpec(t *testing.T) {
	tests := []struct {
		name        string
		operator    r3json.JSONFilterOperator
		expected    r3.FilterOperatorSpec
		expectError bool
	}{
		{
			name:        "OperatorEq",
			operator:    r3json.OperatorEq,
			expected:    r3.OperatorEq,
			expectError: false,
		},
		{
			name:        "OperatorNe",
			operator:    r3json.OperatorNe,
			expected:    r3.OperatorNe,
			expectError: false,
		},
		{
			name:        "OperatorExists",
			operator:    r3json.OperatorExists,
			expected:    r3.OperatorExists,
			expectError: false,
		},
		{
			name:        "OperatorGt",
			operator:    r3json.OperatorGt,
			expected:    r3.OperatorGt,
			expectError: false,
		},
		{
			name:        "OperatorGte",
			operator:    r3json.OperatorGte,
			expected:    r3.OperatorGte,
			expectError: false,
		},
		{
			name:        "OperatorLt",
			operator:    r3json.OperatorLt,
			expected:    r3.OperatorLt,
			expectError: false,
		},
		{
			name:        "OperatorLte",
			operator:    r3json.OperatorLte,
			expected:    r3.OperatorLte,
			expectError: false,
		},
		{
			name:        "OperatorBetween",
			operator:    r3json.OperatorBetween,
			expected:    r3.OperatorBetween,
			expectError: false,
		},
		{
			name:        "OperatorBetweenEx",
			operator:    r3json.OperatorBetweenEx,
			expected:    r3.OperatorBetweenEx,
			expectError: false,
		},
		{
			name:        "OperatorBetweenExInc",
			operator:    r3json.OperatorBetweenExInc,
			expected:    r3.OperatorBetweenExInc,
			expectError: false,
		},
		{
			name:        "OperatorBetweenIncEx",
			operator:    r3json.OperatorBetweenIncEx,
			expected:    r3.OperatorBetweenIncEx,
			expectError: false,
		},
		{
			name:        "OperatorIn",
			operator:    r3json.OperatorIn,
			expected:    r3.OperatorIn,
			expectError: false,
		},
		{
			name:        "OperatorNotIn",
			operator:    r3json.OperatorNotIn,
			expected:    r3.OperatorNotIn,
			expectError: false,
		},
		{
			name:        "OperatorLike",
			operator:    r3json.OperatorLike,
			expected:    r3.OperatorLike,
			expectError: false,
		},
		{
			name:        "OperatorNotLike",
			operator:    r3json.OperatorNotLike,
			expected:    r3.OperatorNotLike,
			expectError: false,
		},
		{
			name:        "OperatorILike",
			operator:    r3json.OperatorILike,
			expected:    r3.OperatorILike,
			expectError: false,
		},
		{
			name:        "OperatorUnspecified",
			operator:    r3json.OperatorUnspecified,
			expected:    r3.OperatorUnspecified,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.operator.ToFilterOperatorSpec()

			if tt.expectError {
				require.Error(t, err)
				assert.Equal(t, r3.OperatorUnspecified, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFilterOperatorValues_CompleteCoverage(t *testing.T) {
	// Test that FilterOperatorValues map contains all expected keys
	expectedKeys := []string{
		"", "eq", "==", "ne", "!=", "exists", "gt", ">", "gte", ">=",
		"lt", "<", "lte", "<=", "between", "between_inc", "between_exc",
		"between_exc_inc", "between_inc_exc", "in", "nin", "like", "*=",
		"notlike", "!*=", "ilike", "**=",
	}

	for _, key := range expectedKeys {
		_, exists := r3json.FilterOperatorValues[key]
		assert.True(t, exists, "FilterOperatorValues should contain key: %s", key)
	}
}

func TestFilterOperatorNames_CompleteCoverage(t *testing.T) {
	// Test that FilterOperatorNames map contains all operator constants
	operators := []r3json.JSONFilterOperator{
		r3json.OperatorUnspecified, r3json.OperatorEq, r3json.OperatorNe, r3json.OperatorExists,
		r3json.OperatorGt, r3json.OperatorGte, r3json.OperatorLt, r3json.OperatorLte,
		r3json.OperatorBetween, r3json.OperatorBetweenEx, r3json.OperatorBetweenExInc, r3json.OperatorBetweenIncEx,
		r3json.OperatorIn, r3json.OperatorNotIn, r3json.OperatorLike, r3json.OperatorNotLike, r3json.OperatorILike,
	}

	for _, op := range operators {
		_, exists := r3json.FilterOperatorNames[op]
		assert.True(t, exists, "FilterOperatorNames should contain operator: %d", op)
	}
}

func TestJSONFilterOperator_JSONRoundTrip(t *testing.T) {
	testOperators := []r3json.JSONFilterOperator{
		r3json.OperatorEq, r3json.OperatorNe, r3json.OperatorGt, r3json.OperatorLike, r3json.OperatorBetween,
	}

	for _, original := range testOperators {
		t.Run(original.String(), func(t *testing.T) {
			// Marshal to JSON
			jsonData, err := json.Marshal(original)
			require.NoError(t, err)

			// Unmarshal from JSON
			var unmarshaled r3json.JSONFilterOperator
			err = json.Unmarshal(jsonData, &unmarshaled)
			require.NoError(t, err)

			// Should be equal
			assert.Equal(t, original, unmarshaled)
		})
	}
}

func TestJSONFilterOperator_TextRoundTrip(t *testing.T) {
	testOperators := []r3json.JSONFilterOperator{
		r3json.OperatorEq, r3json.OperatorNe, r3json.OperatorGt, r3json.OperatorLike, r3json.OperatorBetween,
	}

	for _, original := range testOperators {
		t.Run(original.String(), func(t *testing.T) {
			// Marshal to text
			textData, err := original.MarshalText()
			require.NoError(t, err)

			// Unmarshal from text
			var unmarshaled r3json.JSONFilterOperator
			err = unmarshaled.UnmarshalText(textData)
			require.NoError(t, err)

			// Should be equal
			assert.Equal(t, original, unmarshaled)
		})
	}
}
