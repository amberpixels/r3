package r3_test

import (
	"testing"

	"github.com/amberpixels/k1/maybe"
	"github.com/amberpixels/r3"
	"github.com/expectto/be"
	"github.com/expectto/be/be_math"
	"github.com/expectto/be/be_struct"
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

			be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", tt.expected.PageNum))
			be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", tt.expected.PageSize))
		})
	}
}

func TestNewPaginationSpecWithSize(t *testing.T) {
	result := r3.NewPaginationSpecWithSize(100)

	be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", maybe.None[int]()))
	be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", maybe.Some(100)))
}

func TestNoPagination(t *testing.T) {
	result := r3.NoPagination()

	be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", maybe.None[int]()))
	be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", maybe.None[int]()))
	be.AssertThat(t, result.IsPaginated(), be.False())
}

func TestDefaultPagination(t *testing.T) {
	result := r3.DefaultPagination()

	be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", maybe.None[int]()))
	be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", maybe.Some(r3.PageSizeDefault)))
	be.AssertThat(t, result.IsPaginated(), be.True())
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
			be.AssertThat(t, result, be.Eq(tt.expected))
		})
	}
}

func TestPaginationSpec_Clone(t *testing.T) {
	original := &r3.PaginationSpec{
		PageNum:  maybe.Some(2),
		PageSize: maybe.Some(25),
	}

	cloned := original.Clone()

	// Verify values are equal
	be.AssertThat(t, cloned, be_struct.HavingField[r3.PaginationSpec]("PageNum", original.PageNum))
	be.AssertThat(t, cloned, be_struct.HavingField[r3.PaginationSpec]("PageSize", original.PageSize))

	// Verify it's a different instance
	assertNotSamePtr(t, original, cloned)
}

// assertNotSamePtr fails if a and b are the same pointer. be.NotIdentical is
// the identity check equivalent to testify's assert.NotSame (be.Ne would
// instead compare by VALUE and false-fail here since Clone copies the fields).
func assertNotSamePtr[T any](t *testing.T, a, b *T) {
	t.Helper()
	be.AssertThat(t, a, be.NotIdentical(b))
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
			be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", tt.expected.PageNum))
			be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", tt.expected.PageSize))
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
			be.AssertThat(t, result, be.Eq(tt.expected))
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
			be.AssertThat(t, result, be.Eq(tt.expected))
			// Page numbers are 1-indexed: the getter must never return < 1.
			be.AssertThat(t, result, be_math.Gte(1))
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
			be.AssertThat(t, result, be.Eq(tt.expected))
			// A resolved page size is always a positive count.
			be.AssertThat(t, result, be_math.Positive())
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
			be.AssertThat(t, limit, be.Eq(tt.expectedLimit))
			be.AssertThat(t, offset, be.Eq(tt.expectedOffset))
			// LIMIT must be a positive row count; OFFSET is non-negative.
			be.AssertThat(t, limit, be_math.Positive())
			be.AssertThat(t, offset, be_math.Gte(0))
		})
	}
}

func TestNewOffsetPagination(t *testing.T) {
	t.Run("offset and limit", func(t *testing.T) {
		result := r3.NewOffsetPagination(100, 30)
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("Offset", maybe.Some(100)))
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", maybe.Some(30)))
		// Offset is an alternative to PageNum, so PageNum stays unset.
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", maybe.None[int]()))
	})

	t.Run("offset only defaults the size", func(t *testing.T) {
		result := r3.NewOffsetPagination(40)
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("Offset", maybe.Some(40)))
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", maybe.None[int]()))
		be.AssertThat(t, result.GetPageSize(), be.Eq(r3.PageSizeDefault))
	})
}

func TestPaginationSpec_GetOffset(t *testing.T) {
	tests := []struct {
		name       string
		pageSpec   *r3.PaginationSpec
		wantOffset int
		wantOK     bool
	}{
		{"offset set", &r3.PaginationSpec{Offset: maybe.Some(100)}, 100, true},
		{"offset zero", &r3.PaginationSpec{Offset: maybe.Some(0)}, 0, true},
		{"offset negative clamps to 0", &r3.PaginationSpec{Offset: maybe.Some(-5)}, 0, true},
		{"offset unset", &r3.PaginationSpec{PageNum: maybe.Some(2)}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, ok := tt.pageSpec.GetOffset()
			be.AssertThat(t, offset, be.Eq(tt.wantOffset))
			be.AssertThat(t, ok, be.Eq(tt.wantOK))
		})
	}
}

// TestPaginationSpec_ToLimitOffset_Offset covers the raw-offset path: the offset
// reaches the driver verbatim (no page rounding), so a starting row that is not a
// multiple of the limit - the whole point of the feature - is expressible.
func TestPaginationSpec_ToLimitOffset_Offset(t *testing.T) {
	tests := []struct {
		name           string
		pageSpec       *r3.PaginationSpec
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "misaligned offset survives (100, 30)",
			pageSpec:       r3.NewOffsetPagination(100, 30),
			expectedLimit:  30,
			expectedOffset: 100, // NOT rounded to a page boundary (90)
		},
		{
			name:           "offset takes precedence over page number",
			pageSpec:       &r3.PaginationSpec{Offset: maybe.Some(15), PageNum: maybe.Some(9), PageSize: maybe.Some(5)},
			expectedLimit:  5,
			expectedOffset: 15,
		},
		{
			name:           "offset only defaults the limit",
			pageSpec:       r3.NewOffsetPagination(7),
			expectedLimit:  r3.PageSizeDefault,
			expectedOffset: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset := tt.pageSpec.ToLimitOffset()
			be.AssertThat(t, limit, be.Eq(tt.expectedLimit))
			be.AssertThat(t, offset, be.Eq(tt.expectedOffset))
		})
	}
}

// TestPaginationSpec_MergeWith_PositionExclusive: PageNum and Offset are mutually
// exclusive position encodings, so an override of one clears the other - a stale
// base position must never shadow the newer intent.
func TestPaginationSpec_MergeWith_PositionExclusive(t *testing.T) {
	t.Run("offset override clears base page", func(t *testing.T) {
		base := &r3.PaginationSpec{PageNum: maybe.Some(3), PageSize: maybe.Some(25)}
		result := base.MergeWith(r3.NewOffsetPagination(100))
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("Offset", maybe.Some(100)))
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", maybe.None[int]()))
		// PageSize is independent and is preserved from the base.
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", maybe.Some(25)))
	})

	t.Run("page override clears base offset", func(t *testing.T) {
		base := r3.NewOffsetPagination(100, 30)
		result := base.MergeWith(r3.NewPaginationSpec(2))
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageNum", maybe.Some(2)))
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("Offset", maybe.None[int]()))
		be.AssertThat(t, result, be_struct.HavingField[r3.PaginationSpec]("PageSize", maybe.Some(30)))
	})
}

func TestPaginationSpec_Offset_StringCloneIsPaginated(t *testing.T) {
	t.Run("String with size", func(t *testing.T) {
		be.AssertThat(t, r3.NewOffsetPagination(100, 30).String(), be.Eq("offset=100,size=30"))
	})
	t.Run("String offset only", func(t *testing.T) {
		be.AssertThat(t, (&r3.PaginationSpec{Offset: maybe.Some(40)}).String(), be.Eq("offset=40"))
	})
	t.Run("Clone preserves offset", func(t *testing.T) {
		be.AssertThat(t, r3.NewOffsetPagination(100, 30).Clone(),
			be_struct.HavingField[r3.PaginationSpec]("Offset", maybe.Some(100)))
	})
	t.Run("IsPaginated on offset-only spec", func(t *testing.T) {
		be.AssertThat(t, (&r3.PaginationSpec{Offset: maybe.Some(5)}).IsPaginated(), be.True())
	})
}
