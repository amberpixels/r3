package r3url_test

import (
	"net/url"
	"testing"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePagination(t *testing.T) {
	params := r3url.DefaultParamNames()

	tests := []struct {
		name         string
		values       url.Values
		expectedPage int
		expectedSize int
		isNil        bool
		expectError  bool
	}{
		{
			name:         "both page and page_size",
			values:       url.Values{"page": {"2"}, "page_size": {"25"}},
			expectedPage: 2,
			expectedSize: 25,
		},
		{
			name:         "only page",
			values:       url.Values{"page": {"3"}},
			expectedPage: 3,
		},
		{
			name:         "only page_size",
			values:       url.Values{"page_size": {"50"}},
			expectedSize: 50,
		},
		{
			name:   "no pagination params",
			values: url.Values{},
			isNil:  true,
		},
		{
			name:        "invalid page number",
			values:      url.Values{"page": {"abc"}},
			expectError: true,
		},
		{
			name:        "invalid page_size",
			values:      url.Values{"page_size": {"xyz"}},
			expectError: true,
		},
		{
			name:        "negative page",
			values:      url.Values{"page": {"-1"}},
			expectError: true,
		},
		{
			name:        "negative page_size",
			values:      url.Values{"page_size": {"-5"}},
			expectError: true,
		},
		{
			name:   "zero values",
			values: url.Values{"page": {"0"}, "page_size": {"0"}},
			isNil:  false, // returns NoPagination
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r3url.ParsePagination(tt.values, params)

			if tt.expectError {
				require.Error(t, err)
				assert.True(t, r3url.IsErrDialectorFailure(err))
				return
			}

			require.NoError(t, err)
			if tt.isNil {
				assert.Nil(t, result)
			} else if tt.expectedPage > 0 || tt.expectedSize > 0 {
				require.NotNil(t, result)
				if tt.expectedPage > 0 {
					assert.Equal(t, tt.expectedPage, result.GetPageNum())
				}
				if tt.expectedSize > 0 {
					assert.Equal(t, tt.expectedSize, result.GetPageSize())
				}
			}
		})
	}
}

func TestFormatPagination(t *testing.T) {
	params := r3url.DefaultParamNames()

	tests := []struct {
		name     string
		input    *r3.PaginationSpec
		expected url.Values
	}{
		{
			name:     "with page and size",
			input:    r3.NewPaginationSpec(2, 25),
			expected: url.Values{"page": {"2"}, "page_size": {"25"}},
		},
		{
			name:     "nil pagination",
			input:    nil,
			expected: url.Values{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3url.FormatPagination(tt.input, params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPaginationRoundTrip(t *testing.T) {
	params := r3url.DefaultParamNames()
	original := r3.NewPaginationSpec(3, 50)

	formatted := r3url.FormatPagination(original, params)
	parsed, err := r3url.ParsePagination(formatted, params)
	require.NoError(t, err)
	require.NotNil(t, parsed)

	assert.Equal(t, original.GetPageNum(), parsed.GetPageNum())
	assert.Equal(t, original.GetPageSize(), parsed.GetPageSize())
}
