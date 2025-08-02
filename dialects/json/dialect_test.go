package r3json_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONInboundDialector_TranslateIntoFieldSpec(t *testing.T) {
	dialector := &r3json.JSONInboundDialector{}

	tests := []struct {
		name          string
		input         r3.DialectValue
		expectedField string
		expectError   bool
	}{
		{
			name:          "valid JSONField value",
			input:         r3json.JSONField("id"),
			expectedField: "id",
			expectError:   false,
		},
		{
			name:          "valid JSONField pointer",
			input:         func() *r3json.JSONField { f := r3json.JSONField("status"); return &f }(),
			expectedField: "status",
			expectError:   false,
		},
		{
			name:        "invalid type",
			input:       "not a JSONField",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoFieldSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.True(t, r3json.IsErrDialectorFailure(err))
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedField, result.String())
			}
		})
	}
}

func TestJSONInboundDialector_TranslateIntoFilterSpec(t *testing.T) {
	dialector := &r3json.JSONInboundDialector{}

	tests := []struct {
		name        string
		input       r3.DialectValue
		expectError bool
	}{
		{
			name: "valid JSONFilter",
			input: &r3json.JSONFilter{
				Field: r3json.JSONField("id"),
				Op:    r3json.OperatorEq,
				Value: 123,
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			input:       "not a JSONFilter",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoFilterSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.True(t, r3json.IsErrDialectorFailure(err))
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestJSONInboundDialector_TranslateIntoFilterOperatorSpec(t *testing.T) {
	dialector := &r3json.JSONInboundDialector{}

	tests := []struct {
		name             string
		input            r3.DialectValue
		expectedOperator r3.FilterOperatorSpec
		expectError      bool
	}{
		{
			name:             "valid JSONFilterOperator value",
			input:            r3json.OperatorEq,
			expectedOperator: r3.OperatorEq,
			expectError:      false,
		},
		{
			name:             "valid JSONFilterOperator pointer",
			input:            func() *r3json.JSONFilterOperator { op := r3json.OperatorGt; return &op }(),
			expectedOperator: r3.OperatorGt,
			expectError:      false,
		},
		{
			name:             "valid string input",
			input:            "eq",
			expectedOperator: r3.OperatorEq,
			expectError:      false,
		},
		{
			name:        "invalid string input",
			input:       "invalid_operator",
			expectError: true,
		},
		{
			name:        "invalid type",
			input:       123,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoFilterOperatorSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.True(t, r3json.IsErrDialectorFailure(err))
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedOperator, *result)
			}
		})
	}
}

func TestJSONOutboundDialector_TranslateFieldSpec(t *testing.T) {
	dialector := &r3json.JSONOutboundDialector{}

	tests := []struct {
		name        string
		input       *r3.FieldSpec
		expected    string
		expectError bool
	}{
		{
			name:        "valid field spec",
			input:       r3.NewFieldSpec("user_id"),
			expected:    "user_id",
			expectError: false,
		},
		{
			name:        "nil field spec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateFieldSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.True(t, r3json.IsErrDialectorFailure(err))
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				jsonField, ok := result.(r3json.JSONField)
				require.True(t, ok)
				assert.Equal(t, tt.expected, string(jsonField))
			}
		})
	}
}

func TestJSONOutboundDialector_TranslateFilterSpec(t *testing.T) {
	dialector := &r3json.JSONOutboundDialector{}

	tests := []struct {
		name        string
		input       *r3.FilterSpec
		expectError bool
	}{
		{
			name: "valid simple filter",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("id"),
				Operator: r3.OperatorEq,
				Value:    123,
			},
			expectError: false,
		},
		{
			name: "filter with AND groups - not supported",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("id"),
				Operator: r3.OperatorEq,
				Value:    123,
				And:      r3.Filters{&r3.FilterSpec{}},
			},
			expectError: true,
		},
		{
			name: "filter with OR groups - not supported",
			input: &r3.FilterSpec{
				Field:    r3.NewFieldSpec("id"),
				Operator: r3.OperatorEq,
				Value:    123,
				Or:       r3.Filters{&r3.FilterSpec{}},
			},
			expectError: true,
		},
		{
			name:        "nil filter spec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateFilterSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.True(t, r3json.IsErrDialectorFailure(err))
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				jsonFilter, ok := result.(*r3json.JSONFilter)
				require.True(t, ok)
				assert.Equal(t, "id", string(jsonFilter.Field))
				assert.Equal(t, r3json.OperatorEq, jsonFilter.Op)
				assert.Equal(t, 123, jsonFilter.Value)
			}
		})
	}
}

func TestJSONOutboundDialector_TranslateFilterOperatorSpec(t *testing.T) {
	dialector := &r3json.JSONOutboundDialector{}

	tests := []struct {
		name        string
		input       *r3.FilterOperatorSpec
		expected    r3json.JSONFilterOperator
		expectError bool
	}{
		{
			name:        "valid operator",
			input:       func() *r3.FilterOperatorSpec { op := r3.OperatorEq; return &op }(),
			expected:    r3json.OperatorEq,
			expectError: false,
		},
		{
			name:        "nil operator spec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateFilterOperatorSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
				assert.True(t, r3json.IsErrDialectorFailure(err))
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				jsonOp, ok := result.(r3json.JSONFilterOperator)
				require.True(t, ok)
				assert.Equal(t, tt.expected, jsonOp)
			}
		})
	}
}

// func TestR3ToJSONOperator(t *testing.T) {
// 	tests := []struct {
// 		name        string
// 		input       r3.FilterOperatorSpec
// 		expected    r3json.JSONFilterOperator
// 		expectError bool
// 	}{
// 		{
// 			name:        "OperatorEq",
// 			input:       r3.OperatorEq,
// 			expected:    r3json.OperatorEq,
// 			expectError: false,
// 		},
// 		{
// 			name:        "OperatorNe",
// 			input:       r3.OperatorNe,
// 			expected:    r3json.OperatorNe,
// 			expectError: false,
// 		},
// 		{
// 			name:        "OperatorGt",
// 			input:       r3.OperatorGt,
// 			expected:    r3json.OperatorGt,
// 			expectError: false,
// 		},
// 		{
// 			name:        "OperatorLike",
// 			input:       r3.OperatorLike,
// 			expected:    r3json.OperatorLike,
// 			expectError: false,
// 		},
// 		{
// 			name:        "OperatorUnspecified",
// 			input:       r3.OperatorUnspecified,
// 			expected:    r3json.OperatorUnspecified,
// 			expectError: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result, err := r3json.r3ToJSONOperator(tt.input)

// 			if tt.expectError {
// 				require.Error(t, err)
// 			} else {
// 				require.NoError(t, err)
// 				assert.Equal(t, tt.expected, result)
// 			}
// 		})
// 	}
// }
