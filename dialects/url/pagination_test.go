package r3url_test

import (
	"net/url"
	"testing"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
	"github.com/expectto/be"
	"github.com/expectto/be/be_reflected"
	"github.com/expectto/be/be_url"
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
				be.Error(t, err)
				be.AssertThat(t, r3url.IsErrDialectorFailure(err), be.True())
				return
			}

			be.NoError(t, err)
			if tt.isNil {
				be.AssertThat(t, result, be.Nil())
			} else if tt.expectedPage > 0 || tt.expectedSize > 0 {
				be.RequireThat(t, result, be.NotNil())
				if tt.expectedPage > 0 {
					be.AssertThat(t, result.GetPageNum(), be.Eq(tt.expectedPage))
				}
				if tt.expectedSize > 0 {
					be.AssertThat(t, result.GetPageSize(), be.Eq(tt.expectedSize))
				}
			}
		})
	}
}

func TestFormatPagination(t *testing.T) {
	params := r3url.DefaultParamNames()

	t.Run("with page and size", func(t *testing.T) {
		result := r3url.FormatPagination(r3.NewPaginationSpec(2, 25), params)

		// Build a *url.URL from the produced url.Values so be_url matchers apply.
		u := &url.URL{RawQuery: result.Encode()}
		be.AssertThat(t, u, be_url.URL(
			// plain-value form
			be_url.HavingSearchParam("page", "2"),
			// matcher-value form: page_size must be a numeric string ("25")
			be_url.HavingSearchParam("page_size", be_reflected.AsNumericString()),
		))
	})

	t.Run("nil pagination", func(t *testing.T) {
		result := r3url.FormatPagination(nil, params)

		// Empty url.Values -> empty raw query, no page params present.
		u := &url.URL{RawQuery: result.Encode()}
		be.AssertThat(t, u, be_url.URL(
			be_url.HavingRawQuery(""),
			be_url.HavingSearchParam("page", ""),
			be_url.HavingSearchParam("page_size", ""),
		))
	})
}

func TestPaginationRoundTrip(t *testing.T) {
	params := r3url.DefaultParamNames()
	original := r3.NewPaginationSpec(3, 50)

	formatted := r3url.FormatPagination(original, params)
	parsed, err := r3url.ParsePagination(formatted, params)
	be.NoError(t, err)
	be.RequireThat(t, parsed, be.NotNil())

	be.AssertThat(t, parsed.GetPageNum(), be.Eq(original.GetPageNum()))
	be.AssertThat(t, parsed.GetPageSize(), be.Eq(original.GetPageSize()))
}

func TestParsePagination_Offset(t *testing.T) {
	params := r3url.DefaultParamNames()

	t.Run("offset and page_size (misaligned)", func(t *testing.T) {
		result, err := r3url.ParsePagination(url.Values{"offset": {"100"}, "page_size": {"30"}}, params)
		be.NoError(t, err)
		be.RequireThat(t, result, be.NotNil())
		offset, ok := result.GetOffset()
		be.AssertThat(t, ok, be.True())
		be.AssertThat(t, offset, be.Eq(100))
		be.AssertThat(t, result.GetPageSize(), be.Eq(30))
	})

	t.Run("offset only defaults the size", func(t *testing.T) {
		result, err := r3url.ParsePagination(url.Values{"offset": {"40"}}, params)
		be.NoError(t, err)
		be.RequireThat(t, result, be.NotNil())
		offset, ok := result.GetOffset()
		be.AssertThat(t, ok, be.True())
		be.AssertThat(t, offset, be.Eq(40))
		be.AssertThat(t, result.GetPageSize(), be.Eq(r3.PageSizeDefault))
	})

	t.Run("offset with zero size falls back to default (not zero rows)", func(t *testing.T) {
		result, err := r3url.ParsePagination(url.Values{"offset": {"40"}, "page_size": {"0"}}, params)
		be.NoError(t, err)
		be.RequireThat(t, result, be.NotNil())
		offset, ok := result.GetOffset()
		be.AssertThat(t, ok, be.True())
		be.AssertThat(t, offset, be.Eq(40))
		be.AssertThat(t, result.GetPageSize(), be.Eq(r3.PageSizeDefault))
	})

	t.Run("page and offset are mutually exclusive", func(t *testing.T) {
		_, err := r3url.ParsePagination(url.Values{"page": {"2"}, "offset": {"10"}}, params)
		be.Error(t, err)
		be.AssertThat(t, r3url.IsErrDialectorFailure(err), be.True())
	})

	t.Run("negative offset", func(t *testing.T) {
		_, err := r3url.ParsePagination(url.Values{"offset": {"-1"}}, params)
		be.Error(t, err)
		be.AssertThat(t, r3url.IsErrDialectorFailure(err), be.True())
	})
}

func TestFormatPagination_Offset(t *testing.T) {
	params := r3url.DefaultParamNames()

	result := r3url.FormatPagination(r3.NewOffsetPagination(100, 30), params)
	u := &url.URL{RawQuery: result.Encode()}
	be.AssertThat(t, u, be_url.URL(
		be_url.HavingSearchParam("offset", "100"),
		be_url.HavingSearchParam("page_size", "30"),
		// An offset-based spec must not leak a page number.
		be_url.HavingSearchParam("page", ""),
	))
}

func TestPaginationRoundTrip_Offset(t *testing.T) {
	params := r3url.DefaultParamNames()
	original := r3.NewOffsetPagination(100, 30) // misaligned: 100 is not a multiple of 30

	formatted := r3url.FormatPagination(original, params)
	parsed, err := r3url.ParsePagination(formatted, params)
	be.NoError(t, err)
	be.RequireThat(t, parsed, be.NotNil())

	gotOffset, ok := parsed.GetOffset()
	be.AssertThat(t, ok, be.True())
	be.AssertThat(t, gotOffset, be.Eq(100))
	be.AssertThat(t, parsed.GetPageSize(), be.Eq(30))
}
