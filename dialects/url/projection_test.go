package r3url_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	r3url "github.com/amberpixels/r3/dialects/url"
)

// fieldNames flattens r3.Fields to plain strings for comparison.
func fieldNames(fields r3.Fields) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = f.String()
	}
	return out
}

// Projection spans both modes, so both round-trips live together: a query that
// formats to one mode and parses back unequal is the bug this guards.
func TestExcludeFieldsRoundTrip(t *testing.T) {
	t.Run("decomposed", func(t *testing.T) {
		q := r3.Query{ExcludeFields: r3.Exclude("blob", "payload")}

		values, err := r3url.FormatQuery(q, r3url.WithMode(r3url.ModeDecomposed))
		require.NoError(t, err)
		assert.Equal(t, "blob,payload", values.Get("exclude_fields"))
		assert.Empty(t, values.Get("fields"), "the additive param must stay absent")

		back, err := r3url.ParseQuery(values, r3url.WithMode(r3url.ModeDecomposed))
		require.NoError(t, err)
		assert.Equal(t, []string{"blob", "payload"}, fieldNames(back.ExcludeFields))
		assert.Empty(t, back.Fields)
	})

	t.Run("unified", func(t *testing.T) {
		q := r3.Query{ExcludeFields: r3.Exclude("blob", "payload")}

		values, err := r3url.FormatQuery(q, r3url.WithMode(r3url.ModeUnified))
		require.NoError(t, err)
		assert.Contains(t, values.Get("query"), `"exclude_fields"`)

		back, err := r3url.ParseQuery(values, r3url.WithMode(r3url.ModeUnified))
		require.NoError(t, err)
		assert.Equal(t, []string{"blob", "payload"}, fieldNames(back.ExcludeFields))
		assert.Empty(t, back.Fields)
	})

	t.Run("the additive form is untouched", func(t *testing.T) {
		q := r3.Query{Fields: r3.Include("id", "name")}

		values, err := r3url.FormatQuery(q, r3url.WithMode(r3url.ModeDecomposed))
		require.NoError(t, err)
		assert.Equal(t, "id,name", values.Get("fields"))
		assert.Empty(t, values.Get("exclude_fields"))

		back, err := r3url.ParseQuery(values, r3url.WithMode(r3url.ModeDecomposed))
		require.NoError(t, err)
		assert.Equal(t, []string{"id", "name"}, fieldNames(back.Fields))
		assert.Empty(t, back.ExcludeFields)
	})
}

// Naming both forms is a conflict, not a precedence for the dialect to invent:
// Mongo rejects a mixed projection outright and SQL would have to guess.
func TestBothProjectionFormsConflict(t *testing.T) {
	t.Run("decomposed", func(t *testing.T) {
		values := url.Values{
			"fields":         {"id,name"},
			"exclude_fields": {"blob"},
		}
		_, err := r3url.ParseQuery(values, r3url.WithMode(r3url.ModeDecomposed))
		require.ErrorIs(t, err, r3.ErrProjectionConflict)
		assert.True(t, r3url.IsErrDialectorFailure(err), "still reads as a dialect failure")
	})

	t.Run("unified", func(t *testing.T) {
		values := url.Values{
			"query": {`{"fields":["id"],"exclude_fields":["blob"]}`},
		}
		_, err := r3url.ParseQuery(values, r3url.WithMode(r3url.ModeUnified))
		require.ErrorIs(t, err, r3.ErrProjectionConflict)
	})

	t.Run("auto", func(t *testing.T) {
		values := url.Values{
			"fields":         {"id,name"},
			"exclude_fields": {"blob"},
		}
		_, err := r3url.ParseQuery(values)
		require.ErrorIs(t, err, r3.ErrProjectionConflict)
	})

	// Formatting applies the same rule, or the dialect would emit a URL it
	// refuses to read back.
	t.Run("formatting rejects it too", func(t *testing.T) {
		q := r3.Query{Fields: r3.Include("id"), ExcludeFields: r3.Exclude("blob")}

		_, err := r3url.FormatQuery(q, r3url.WithMode(r3url.ModeDecomposed))
		require.ErrorIs(t, err, r3.ErrProjectionConflict)

		_, err = r3url.FormatQuery(q, r3url.WithMode(r3url.ModeUnified))
		require.ErrorIs(t, err, r3.ErrProjectionConflict)
	})
}

// The dialect's own params are kept out of the Django-style filter parser. A
// param missing from that set is read as a filter on a column of the same name,
// which is a silently wrong query rather than an error.
//
// WithDjangoStyleFilters is called with no whitelist on purpose: an explicit one
// would exclude exclude_fields by itself and this would assert nothing. With the
// whitelist empty (every field allowed), reservedParamNames is the only guard.
func TestExcludeFieldsIsNotADjangoFilter(t *testing.T) {
	values := url.Values{
		"exclude_fields": {"blob"},
		"status":         {"active"},
	}

	q, err := r3url.ParseQuery(values,
		r3url.WithMode(r3url.ModeDecomposed),
		r3url.WithDjangoStyleFilters(),
	)
	require.NoError(t, err)

	assert.Equal(t, []string{"blob"}, fieldNames(q.ExcludeFields))
	require.Len(t, q.Filters, 1, "only status is a filter; exclude_fields is a reserved param")
	assert.Equal(t, "status", q.Filters[0].Field.String())
}
