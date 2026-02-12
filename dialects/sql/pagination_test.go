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

func TestPaginationToSQL_Spec(t *testing.T) {
	tests := []struct {
		name        string
		input       *r3.PaginationSpec
		expectedSQL *r3sql.SQLPagination
		isNil       bool
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
			name:  "nil pagination spec",
			input: nil,
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3sql.PaginationToSQL(tt.input)

			if tt.isNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedSQL.Limit, result.Limit)
				assert.Equal(t, tt.expectedSQL.Offset, result.Offset)
			}
		})
	}
}

func TestSQLToPagination_Spec(t *testing.T) {
	tests := []struct {
		name         string
		input        *r3sql.SQLPagination
		expectedSpec *r3.PaginationSpec
		isNil        bool
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
			name:  "nil input",
			input: nil,
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r3sql.SQLToPagination(tt.input)

			if tt.isNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedSpec.GetPageNum(), result.GetPageNum())
				assert.Equal(t, tt.expectedSpec.GetPageSize(), result.GetPageSize())
			}
		})
	}
}

func TestSQLPagination_RoundTrip(t *testing.T) {
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
			sqlPagination := r3sql.PaginationToSQL(tt.original)
			require.NotNil(t, sqlPagination)

			// Convert back to PaginationSpec
			result := r3sql.SQLToPagination(sqlPagination)
			require.NotNil(t, result)

			// Verify round-trip consistency
			assert.Equal(t, tt.original.GetPageNum(), result.GetPageNum())
			assert.Equal(t, tt.original.GetPageSize(), result.GetPageSize())
		})
	}
}
