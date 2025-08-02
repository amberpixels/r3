package r3json_test

import (
	"encoding/json"
	"testing"

	"github.com/amberpixels/r3"
	r3json "github.com/amberpixels/r3/dialects/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPagination_String(t *testing.T) {
	tests := []struct {
		name       string
		pagination *r3json.JSONPagination
		expected   string
	}{
		{
			name: "page number and page size",
			pagination: &r3json.JSONPagination{
				PageNum:  2,
				PageSize: 25,
			},
			expected: `{"page_num":2,"page_size":25}`,
		},
		{
			name: "page number only",
			pagination: &r3json.JSONPagination{
				PageNum: 3,
			},
			expected: `{"page_num":3}`,
		},
		{
			name: "page size only",
			pagination: &r3json.JSONPagination{
				PageSize: 100,
			},
			expected: `{"page_size":100}`,
		},
		{
			name:       "empty pagination",
			pagination: &r3json.JSONPagination{},
			expected:   `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pagination.String()
			assert.JSONEq(t, tt.expected, result)
		})
	}
}

func TestJSONPagination_ToPaginationSpec(t *testing.T) {
	tests := []struct {
		name        string
		pagination  *r3json.JSONPagination
		expected    *r3.PaginationSpec
		expectError bool
	}{
		{
			name: "valid pagination with both values",
			pagination: &r3json.JSONPagination{
				PageNum:  3,
				PageSize: 25,
			},
			expected: r3.NewPaginationSpec(3, 25),
		},
		{
			name: "page number only",
			pagination: &r3json.JSONPagination{
				PageNum: 2,
			},
			expected: r3.NewPaginationSpec(2),
		},
		{
			name: "page size only",
			pagination: &r3json.JSONPagination{
				PageSize: 100,
			},
			expected: r3.NewPaginationSpecWithSize(100),
		},
		{
			name:       "empty pagination",
			pagination: &r3json.JSONPagination{},
			expected:   r3.NoPagination(),
		},
		{
			name: "negative values",
			pagination: &r3json.JSONPagination{
				PageNum:  -1,
				PageSize: -10,
			},
			expected: r3.NoPagination(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.pagination.ToPaginationSpec()

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.GetPageNum(), result.GetPageNum())
				assert.Equal(t, tt.expected.GetPageSize(), result.GetPageSize())
			}
		})
	}
}

func TestJSONPagination_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name       string
		pagination *r3json.JSONPagination
		expected   string
	}{
		{
			name: "full pagination specification",
			pagination: &r3json.JSONPagination{
				PageNum:  4,
				PageSize: 50,
			},
			expected: `{"page_num":4,"page_size":50}`,
		},
		{
			name: "minimal pagination specification",
			pagination: &r3json.JSONPagination{
				PageNum: 1,
			},
			expected: `{"page_num":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.pagination)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(jsonData))

			// Test unmarshaling
			var unmarshaledPagination r3json.JSONPagination
			err = json.Unmarshal(jsonData, &unmarshaledPagination)
			require.NoError(t, err)
			assert.Equal(t, tt.pagination.PageNum, unmarshaledPagination.PageNum)
			assert.Equal(t, tt.pagination.PageSize, unmarshaledPagination.PageSize)
		})
	}
}

func TestJSONInboundDialector_TranslateIntoPaginationSpec(t *testing.T) {
	dialector := &r3json.JSONInboundDialector{}

	tests := []struct {
		name        string
		input       r3.DialectValue
		expected    *r3.PaginationSpec
		expectError bool
	}{
		{
			name: "valid JSONPagination",
			input: &r3json.JSONPagination{
				PageNum:  2,
				PageSize: 30,
			},
			expected: r3.NewPaginationSpec(2, 30),
		},
		{
			name:        "invalid input type",
			input:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoPaginationSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.GetPageNum(), result.GetPageNum())
				assert.Equal(t, tt.expected.GetPageSize(), result.GetPageSize())
			}
		})
	}
}

func TestJSONInboundDialector_TranslatePaginationSpec(t *testing.T) {
	dialector := &r3json.JSONInboundDialector{}

	tests := []struct {
		name        string
		input       *r3.PaginationSpec
		expected    *r3json.JSONPagination
		expectError bool
	}{
		{
			name:  "valid PaginationSpec",
			input: r3.NewPaginationSpec(3, 40),
			expected: &r3json.JSONPagination{
				PageNum:  3,
				PageSize: 40,
			},
		},
		{
			name:        "nil PaginationSpec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslatePaginationSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				jsonPagination, ok := result.(*r3json.JSONPagination)
				require.True(t, ok, "Result should be a JSONPagination")
				assert.Equal(t, tt.expected.PageNum, jsonPagination.PageNum)
				assert.Equal(t, tt.expected.PageSize, jsonPagination.PageSize)
			}
		})
	}
}

func TestJSONOutboundDialector_TranslatePaginationSpec(t *testing.T) {
	dialector := &r3json.JSONOutboundDialector{}

	tests := []struct {
		name        string
		input       *r3.PaginationSpec
		expected    *r3json.JSONPagination
		expectError bool
	}{
		{
			name:  "valid PaginationSpec",
			input: r3.NewPaginationSpec(4, 15),
			expected: &r3json.JSONPagination{
				PageNum:  4,
				PageSize: 15,
			},
		},
		{
			name:        "nil PaginationSpec",
			input:       nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslatePaginationSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				jsonPagination, ok := result.(*r3json.JSONPagination)
				require.True(t, ok, "Result should be a JSONPagination")
				assert.Equal(t, tt.expected.PageNum, jsonPagination.PageNum)
				assert.Equal(t, tt.expected.PageSize, jsonPagination.PageSize)
			}
		})
	}
}

func TestJSONOutboundDialector_TranslateIntoPaginationSpec(t *testing.T) {
	dialector := &r3json.JSONOutboundDialector{}

	tests := []struct {
		name        string
		input       r3.DialectValue
		expected    *r3.PaginationSpec
		expectError bool
	}{
		{
			name: "valid JSONPagination",
			input: &r3json.JSONPagination{
				PageNum:  5,
				PageSize: 20,
			},
			expected: r3.NewPaginationSpec(5, 20),
		},
		{
			name:        "invalid input type",
			input:       42,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := dialector.TranslateIntoPaginationSpec(tt.input)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.GetPageNum(), result.GetPageNum())
				assert.Equal(t, tt.expected.GetPageSize(), result.GetPageSize())
			}
		})
	}
}

func TestJSONPagination_RoundTrip(t *testing.T) {
	inboundDialector := &r3json.JSONInboundDialector{}
	outboundDialector := &r3json.JSONOutboundDialector{}

	tests := []struct {
		name     string
		original *r3.PaginationSpec
	}{
		{
			name:     "page 3, size 25",
			original: r3.NewPaginationSpec(3, 25),
		},
		{
			name:     "page 1, size 100",
			original: r3.NewPaginationSpec(1, 100),
		},
		{
			name:     "default pagination",
			original: r3.DefaultPagination(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to JSON
			jsonValue, err := outboundDialector.TranslatePaginationSpec(tt.original)
			require.NoError(t, err)

			// Convert back to PaginationSpec
			result, err := inboundDialector.TranslateIntoPaginationSpec(jsonValue)
			require.NoError(t, err)

			// Verify round-trip consistency
			assert.Equal(t, tt.original.GetPageNum(), result.GetPageNum())
			assert.Equal(t, tt.original.GetPageSize(), result.GetPageSize())
		})
	}
}
