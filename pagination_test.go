package r3_test

import (
	"testing"

	"github.com/amberpixels/k1/maybe"
	"github.com/amberpixels/r3"
	"github.com/stretchr/testify/assert"
)

func TestNewPaginationSpec(t *testing.T) {
	tests := []struct {
		name     string
		pageNum  int
		pageSize []int
		expected *r3.PaginationSpec
	}{
		{
			name:    "page number only",
			pageNum: 2,
			expected: &r3.PaginationSpec{
				PageNum: maybe.Some(2),
			},
		},
		{
			name:     "page number and page size",
			pageNum:  3,
			pageSize: []int{25},
			expected: &r3.PaginationSpec{
				PageNum:  maybe.Some(3),
				PageSize: maybe.Some(25),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *r3.PaginationSpec
			if len(tt.pageSize) > 0 {
				result = r3.NewPaginationSpec(tt.pageNum, tt.pageSize[0])
			} else {
				result = r3.NewPaginationSpec(tt.pageNum)
			}

			assert.Equal(t, tt.expected.PageNum, result.PageNum)
			assert.Equal(t, tt.expected.PageSize, result.PageSize)
		})
	}
}

func TestNewPaginationSpecWithSize(t *testing.T) {
	result := r3.NewPaginationSpecWithSize(100)

	assert.Equal(t, maybe.None[int](), result.PageNum)
	assert.Equal(t, maybe.Some(100), result.PageSize)
}

func TestNoPagination(t *testing.T) {
	result := r3.NoPagination()

	assert.Equal(t, maybe.None[int](), result.PageNum)
	assert.Equal(t, maybe.None[int](), result.PageSize)
	assert.False(t, result.IsPaginated())
}

func TestDefaultPagination(t *testing.T) {
	result := r3.DefaultPagination()

	assert.Equal(t, maybe.None[int](), result.PageNum)
	assert.Equal(t, maybe.Some(r3.PageSizeDefault), result.PageSize)
	assert.True(t, result.IsPaginated())
}

func TestPaginationSpec_String(t *testing.T) {
	tests := []struct {
		name     string
		pageSpec *r3.PaginationSpec
		expected string
	}{
		{
			name: "page and size specified",
			pageSpec: &r3.PaginationSpec{
				PageNum:  maybe.Some(2),
				PageSize: maybe.Some(25),
			},
			expected: "page=2,size=25",
		},
		{
			name: "page only",
			pageSpec: &r3.PaginationSpec{
				PageNum: maybe.Some(3),
			},
			expected: "page=3",
		},
		{
			name: "size only",
			pageSpec: &r3.PaginationSpec{
				PageSize: maybe.Some(100),
			},
			expected: "size=100",
		},
		{
			name:     "no pagination",
			pageSpec: &r3.PaginationSpec{},
			expected: "no_pagination",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pageSpec.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPaginationSpec_Clone(t *testing.T) {
	original := &r3.PaginationSpec{
		PageNum:  maybe.Some(2),
		PageSize: maybe.Some(25),
	}

	cloned := original.Clone()
	clonedSpec, ok := cloned.(*r3.PaginationSpec)
	assert.True(t, ok)

	// Verify values are equal
	assert.Equal(t, original.PageNum, clonedSpec.PageNum)
	assert.Equal(t, original.PageSize, clonedSpec.PageSize)

	// Verify it's a different instance
	assert.NotSame(t, original, clonedSpec)
}

func TestPaginationSpec_MergeWith(t *testing.T) {
	tests := []struct {
		name     string
		base     *r3.PaginationSpec
		other    *r3.PaginationSpec
		expected *r3.PaginationSpec
	}{
		{
			name: "merge page number",
			base: &r3.PaginationSpec{
				PageSize: maybe.Some(25),
			},
			other: &r3.PaginationSpec{
				PageNum: maybe.Some(3),
			},
			expected: &r3.PaginationSpec{
				PageNum:  maybe.Some(3),
				PageSize: maybe.Some(25),
			},
		},
		{
			name: "override page number",
			base: &r3.PaginationSpec{
				PageNum:  maybe.Some(1),
				PageSize: maybe.Some(25),
			},
			other: &r3.PaginationSpec{
				PageNum: maybe.Some(5),
			},
			expected: &r3.PaginationSpec{
				PageNum:  maybe.Some(5),
				PageSize: maybe.Some(25),
			},
		},
		{
			name: "override page size",
			base: &r3.PaginationSpec{
				PageNum:  maybe.Some(2),
				PageSize: maybe.Some(25),
			},
			other: &r3.PaginationSpec{
				PageSize: maybe.Some(100),
			},
			expected: &r3.PaginationSpec{
				PageNum:  maybe.Some(2),
				PageSize: maybe.Some(100),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.base.MergeWith(tt.other)
			assert.Equal(t, tt.expected.PageNum, result.PageNum)
			assert.Equal(t, tt.expected.PageSize, result.PageSize)
		})
	}
}

func TestPaginationSpec_IsPaginated(t *testing.T) {
	tests := []struct {
		name     string
		pageSpec *r3.PaginationSpec
		expected bool
	}{
		{
			name: "has page number",
			pageSpec: &r3.PaginationSpec{
				PageNum: maybe.Some(2),
			},
			expected: true,
		},
		{
			name: "has page size",
			pageSpec: &r3.PaginationSpec{
				PageSize: maybe.Some(25),
			},
			expected: true,
		},
		{
			name: "has both",
			pageSpec: &r3.PaginationSpec{
				PageNum:  maybe.Some(2),
				PageSize: maybe.Some(25),
			},
			expected: true,
		},
		{
			name:     "has neither",
			pageSpec: &r3.PaginationSpec{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pageSpec.IsPaginated()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPaginationSpec_GetPageNum(t *testing.T) {
	tests := []struct {
		name     string
		pageSpec *r3.PaginationSpec
		expected int
	}{
		{
			name: "page number set",
			pageSpec: &r3.PaginationSpec{
				PageNum: maybe.Some(5),
			},
			expected: 5,
		},
		{
			name: "page number zero - defaults to 1",
			pageSpec: &r3.PaginationSpec{
				PageNum: maybe.Some(0),
			},
			expected: 1,
		},
		{
			name: "page number negative - defaults to 1",
			pageSpec: &r3.PaginationSpec{
				PageNum: maybe.Some(-1),
			},
			expected: 1,
		},
		{
			name:     "page number not set - defaults to 1",
			pageSpec: &r3.PaginationSpec{},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pageSpec.GetPageNum()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPaginationSpec_GetPageSize(t *testing.T) {
	tests := []struct {
		name     string
		pageSpec *r3.PaginationSpec
		expected int
	}{
		{
			name: "page size set",
			pageSpec: &r3.PaginationSpec{
				PageSize: maybe.Some(100),
			},
			expected: 100,
		},
		{
			name:     "page size not set - defaults to PageSizeDefault",
			pageSpec: &r3.PaginationSpec{},
			expected: r3.PageSizeDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pageSpec.GetPageSize()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPaginationSpec_ToLimitOffset(t *testing.T) {
	tests := []struct {
		name           string
		pageSpec       *r3.PaginationSpec
		expectedLimit  int
		expectedOffset int
	}{
		{
			name: "page 1, size 25",
			pageSpec: &r3.PaginationSpec{
				PageNum:  maybe.Some(1),
				PageSize: maybe.Some(25),
			},
			expectedLimit:  25,
			expectedOffset: 0,
		},
		{
			name: "page 3, size 50",
			pageSpec: &r3.PaginationSpec{
				PageNum:  maybe.Some(3),
				PageSize: maybe.Some(50),
			},
			expectedLimit:  50,
			expectedOffset: 100, // (3-1) * 50
		},
		{
			name: "page 5, size 10",
			pageSpec: &r3.PaginationSpec{
				PageNum:  maybe.Some(5),
				PageSize: maybe.Some(10),
			},
			expectedLimit:  10,
			expectedOffset: 40, // (5-1) * 10
		},
		{
			name:           "defaults applied",
			pageSpec:       &r3.PaginationSpec{},
			expectedLimit:  r3.PageSizeDefault,
			expectedOffset: 0, // (1-1) * PageSizeDefault
		},
		{
			name: "only page size specified",
			pageSpec: &r3.PaginationSpec{
				PageSize: maybe.Some(20),
			},
			expectedLimit:  20,
			expectedOffset: 0, // (1-1) * 20
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset := tt.pageSpec.ToLimitOffset()
			assert.Equal(t, tt.expectedLimit, limit)
			assert.Equal(t, tt.expectedOffset, offset)
		})
	}
}
