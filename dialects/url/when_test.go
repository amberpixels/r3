package r3url_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
)

func TestParseQuery_WhenFilter(t *testing.T) {
	values := url.Values{"when": {"weekends"}}

	q, err := r3url.ParseQuery(values,
		r3url.WithMode(r3url.ModeDecomposed),
		r3url.WithWhenFilter("started_at"),
	)
	require.NoError(t, err)

	require.Len(t, q.Filters, 1)
	assert.Equal(t, r3.OperatorWeekdayIn, q.Filters[0].Operator)
	assert.Equal(t, "started_at", q.Filters[0].Field.String())
	assert.Equal(t, []time.Weekday{time.Saturday, time.Sunday}, q.Filters[0].Value)
}

func TestParseQuery_WhenFilter_MergesWithOtherFilters(t *testing.T) {
	values := url.Values{
		"when":    {"mornings"},
		"filters": {`[{"f":"status","op":"eq","v":"active"}]`},
	}

	q, err := r3url.ParseQuery(values,
		r3url.WithMode(r3url.ModeDecomposed),
		r3url.WithWhenFilter("started_at"),
	)
	require.NoError(t, err)
	assert.Len(t, q.Filters, 2)
}

func TestParseQuery_WhenFilter_UnknownKeywordErrors(t *testing.T) {
	values := url.Values{"when": {"notaword"}}

	_, err := r3url.ParseQuery(values,
		r3url.WithMode(r3url.ModeDecomposed),
		r3url.WithWhenFilter("started_at"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "known terms")
}

func TestParseQuery_WhenFilter_DisabledByDefault(t *testing.T) {
	values := url.Values{"when": {"weekends"}}

	q, err := r3url.ParseQuery(values, r3url.WithMode(r3url.ModeDecomposed))
	require.NoError(t, err)
	assert.Empty(t, q.Filters)
}

func TestParseQuery_WhenFilter_AppliedInUnifiedMode(t *testing.T) {
	// A request carrying both ?query= and ?when= must not silently drop when in
	// unified/auto mode - the opted-in filter is merged on top.
	values := url.Values{
		"query": {`{"filters":[{"f":"status","op":"eq","v":"active"}]}`},
		"when":  {"weekends"},
	}

	q, err := r3url.ParseQuery(values, r3url.WithWhenFilter("started_at"))
	require.NoError(t, err)
	require.Len(t, q.Filters, 2)

	var ops []r3.FilterOperatorSpec
	for _, f := range q.Filters {
		ops = append(ops, f.Operator)
	}
	assert.Contains(t, ops, r3.OperatorWeekdayIn)
}

func TestParseQuery_WhenReservedFromDjango(t *testing.T) {
	// With Django-style filters on, ?when= must be interpreted as a time pattern,
	// not swallowed as a "when" field equality filter.
	values := url.Values{"when": {"weekends"}}

	q, err := r3url.ParseQuery(values,
		r3url.WithMode(r3url.ModeDecomposed),
		r3url.WithDjangoStyleFilters(),
		r3url.WithWhenFilter("started_at"),
	)
	require.NoError(t, err)
	require.Len(t, q.Filters, 1)
	assert.Equal(t, r3.OperatorWeekdayIn, q.Filters[0].Operator)
}
