package r3sql_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3sql "github.com/amberpixels/r3/dialects/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSQLPagination(t *testing.T) {
	tests := []struct {
		name           string
		limit          int
		offset         int
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "basic pagination",
			limit:          25,
			offset:         50,
			expectedLimit:  25,
			expectedOffset: 50,
		},
		{
			name:           "zero values",
			limit:          0,
			offset:         0,
			expectedLimit:  0,
			expectedOffset: 0,
		},
		{
			name:           "limit only",
			limit:          100,
			offset:         0,
			expectedLimit:  100,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3sql.NewSQLPagination(tt.limit, tt.offset)

			assert.Equal(t, tt.expectedLimit, result.Limit)
			assert.Equal(t, tt.expectedOffset, result.Offset)
		})
	}
}

func TestSQLPagination_String(t *testing.T) {
	tests := []struct {
		name       string
		pagination *r3sql.SQLPagination
		expected   string
	}{
		{
			name:       "limit and offset",
			pagination: r3sql.NewSQLPagination(25, 50),
			expected:   "LIMIT 25 OFFSET 50",
		},
		{
			name:       "limit only",
			pagination: r3sql.NewSQLPagination(100, 0),
			expected:   "LIMIT 100",
		},
		{
			name:       "offset only",
			pagination: r3sql.NewSQLPagination(0, 25),
			expected:   "OFFSET 25",
		},
		{
			name:       "no pagination",
			pagination: r3sql.NewSQLPagination(0, 0),
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pagination.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSQLPagination_ToLimitOffset(t *testing.T) {
	pagination := r3sql.NewSQLPagination(50, 100)

	limit, offset := pagination.ToLimitOffset()

	assert.Equal(t, 50, limit)
	assert.Equal(t, 100, offset)
}

func TestSQLDialector_TranslatePaginationSpec(t *testing.T) {
	dialector := &r3sql.SQLDialector{}

	tests := []struct {
		name        string
		input       *r3.PaginationSpec
		expectedSQL *r3sql.SQLPagination
		expectError bool
	}{
		{
			name:        "valid pagination spec",
			input:       r3.NewPaginationSpec(3, 25),
			expectedSQL: r3sql.NewSQLPagination(25, 50), // (3-1)*25 = 50 offset
		},
		{
			name:        "page 1",
			input:       r3.NewPaginationSpec(1, 10),
			expectedSQL: r3sql.NewSQLPagination(10, 0), // (1-1)*10 = 0 offset
		},
		{
			name:        "default pagination",
			input:       r3.DefaultPagination(),
			expectedSQL: r3sql.NewSQLPagination(r3.PageSizeDefault, 0), // page 1 with default size
		},
		{
			name:        "nil pagination spec",
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

				sqlPagination, ok := result.(*r3sql.SQLPagination)
				require.True(t, ok, "Result should be a SQLPagination")
				assert.Equal(t, tt.expectedSQL.Limit, sqlPagination.Limit)
				assert.Equal(t, tt.expectedSQL.Offset, sqlPagination.Offset)
			}
		})
	}
}

func TestSQLDialector_TranslateIntoPaginationSpec(t *testing.T) {
	dialector := &r3sql.SQLDialector{}

	tests := []struct {
		name         string
		input        r3.DialectValue
		expectedSpec *r3.PaginationSpec
		expectError  bool
	}{
		{
			name:         "valid SQL pagination",
			input:        r3sql.NewSQLPagination(25, 50),
			expectedSpec: r3.NewPaginationSpec(3, 25), // offset 50 / limit 25 + 1 = page 3
		},
		{
			name:         "page 1",
			input:        r3sql.NewSQLPagination(10, 0),
			expectedSpec: r3.NewPaginationSpec(1, 10), // offset 0 = page 1
		},
		{
			name:         "SQL pagination value type",
			input:        *r3sql.NewSQLPagination(30, 60),
			expectedSpec: r3.NewPaginationSpec(3, 30), // offset 60 / limit 30 + 1 = page 3
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
				assert.Equal(t, tt.expectedSpec.GetPageNum(), result.GetPageNum())
				assert.Equal(t, tt.expectedSpec.GetPageSize(), result.GetPageSize())
			}
		})
	}
}

func TestSQLInboundDialector_TranslatePaginationSpec(t *testing.T) {
	dialector := &r3sql.SQLInboundDialector{}

	tests := []struct {
		name        string
		input       *r3.PaginationSpec
		expectedSQL *r3sql.SQLPagination
		expectError bool
	}{
		{
			name:        "valid pagination spec",
			input:       r3.NewPaginationSpec(4, 20),
			expectedSQL: r3sql.NewSQLPagination(20, 60), // (4-1)*20 = 60 offset
		},
		{
			name:        "nil pagination spec",
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

				sqlPagination, ok := result.(*r3sql.SQLPagination)
				require.True(t, ok, "Result should be a SQLPagination")
				assert.Equal(t, tt.expectedSQL.Limit, sqlPagination.Limit)
				assert.Equal(t, tt.expectedSQL.Offset, sqlPagination.Offset)
			}
		})
	}
}

func TestSQLInboundDialector_TranslateIntoPaginationSpec(t *testing.T) {
	dialector := &r3sql.SQLInboundDialector{}

	tests := []struct {
		name         string
		input        r3.DialectValue
		expectedSpec *r3.PaginationSpec
		expectError  bool
	}{
		{
			name:         "valid SQL pagination",
			input:        r3sql.NewSQLPagination(15, 30),
			expectedSpec: r3.NewPaginationSpec(3, 15), // offset 30 / limit 15 + 1 = page 3
		},
		{
			name:        "invalid input type",
			input:       123,
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
				assert.Equal(t, tt.expectedSpec.GetPageNum(), result.GetPageNum())
				assert.Equal(t, tt.expectedSpec.GetPageSize(), result.GetPageSize())
			}
		})
	}
}

func TestSQLPagination_RoundTrip(t *testing.T) {
	dialector := &r3sql.SQLDialector{}
	inboundDialector := &r3sql.SQLInboundDialector{}

	tests := []struct {
		name     string
		original *r3.PaginationSpec
	}{
		{
			name:     "page 2, size 25",
			original: r3.NewPaginationSpec(2, 25),
		},
		{
			name:     "page 5, size 10",
			original: r3.NewPaginationSpec(5, 10),
		},
		{
			name:     "default pagination",
			original: r3.DefaultPagination(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to SQL
			sqlValue, err := dialector.TranslatePaginationSpec(tt.original)
			require.NoError(t, err)

			// Convert back to PaginationSpec
			result, err := inboundDialector.TranslateIntoPaginationSpec(sqlValue)
			require.NoError(t, err)

			// Verify round-trip consistency
			assert.Equal(t, tt.original.GetPageNum(), result.GetPageNum())
			assert.Equal(t, tt.original.GetPageSize(), result.GetPageSize())
		})
	}
}
