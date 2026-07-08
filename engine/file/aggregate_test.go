package enginefile_test

import (
	"context"
	"testing"

	"github.com/amberpixels/k1/maybe"
	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedAggregateCities(t *testing.T) *enginefile.BaseCRUD[City, int] {
	t.Helper()
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	for _, c := range []City{
		{Name: "Berlin", Country: "Germany"},
		{Name: "Munich", Country: "Germany"},
		{Name: "Hamburg", Country: "Germany"},
		{Name: "Paris", Country: "France"},
		{Name: "Lyon", Country: "France"},
	} {
		_, err := repo.Create(ctx, c)
		require.NoError(t, err)
	}
	return repo
}

func TestAggregate_GroupedCount(t *testing.T) {
	repo := seedAggregateCities(t)

	rows, err := repo.Aggregate(context.Background(), r3.Query{
		GroupBy:    r3.GroupBy("country"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("n"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	country, _ := rows[0].String("country")
	n, _ := rows[0].Int64("n")
	assert.Equal(t, "Germany", country)
	assert.Equal(t, int64(3), n)

	country, _ = rows[1].String("country")
	n, _ = rows[1].Int64("n")
	assert.Equal(t, "France", country)
	assert.Equal(t, int64(2), n)
}

func TestAggregate_NumericFolds(t *testing.T) {
	repo := newJSONRepo[Pet, string](t, enginefile.UUIDStringIDGen())
	ctx := context.Background()

	for _, p := range []Pet{
		{Name: "Rex", Kind: "dog", Age: 2},
		{Name: "Fido", Kind: "dog", Age: 4},
		{Name: "Tom", Kind: "cat", Age: 6},
	} {
		_, err := repo.Create(ctx, p)
		require.NoError(t, err)
	}

	rows, err := repo.Aggregate(ctx, r3.Query{
		GroupBy: r3.GroupBy("kind"),
		Aggregates: r3.Aggregates{
			r3.AggCount("n"),
			r3.AggSum("age", "total_age"),
			r3.AggAvg("age", "avg_age"),
			r3.AggMin("age", "min_age"),
			r3.AggMax("age", "max_age"),
		},
		Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("kind"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// cat first (asc)
	kind, _ := rows[0].String("kind")
	assert.Equal(t, "cat", kind)

	kind, _ = rows[1].String("kind")
	require.Equal(t, "dog", kind)
	total, _ := rows[1].Int64("total_age")
	avg, _ := rows[1].Float64("avg_age")
	minAge, _ := rows[1].Int64("min_age")
	maxAge, _ := rows[1].Int64("max_age")
	assert.Equal(t, int64(6), total)
	assert.InDelta(t, 3.0, avg, 1e-9)
	assert.Equal(t, int64(2), minAge)
	assert.Equal(t, int64(4), maxAge)
}

func TestAggregate_WholeSetRow(t *testing.T) {
	repo := seedAggregateCities(t)

	rows, err := repo.Aggregate(context.Background(), r3.Query{
		Aggregates: r3.Aggregates{
			r3.AggCount("n"),
			r3.AggCountDistinct("country", "countries"),
		},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "no GroupBy = one whole-set row")

	n, _ := rows[0].Int64("n")
	countries, _ := rows[0].Int64("countries")
	assert.Equal(t, int64(5), n)
	assert.Equal(t, int64(2), countries)
}

func TestAggregate_WholeSetRow_EmptyStore(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())

	rows, err := repo.Aggregate(context.Background(), r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "COUNT(*) without GROUP BY yields a row even over zero records")
	n, _ := rows[0].Int64("n")
	assert.Equal(t, int64(0), n)
}

func TestAggregate_FiltersAndHaving(t *testing.T) {
	repo := seedAggregateCities(t)

	// Filter first (drop Berlin), then HAVING n > 1 keeps only Germany... no:
	// Germany keeps Munich+Hamburg (2), France keeps Paris+Lyon (2) — both pass;
	// tighten to n > 2 to drop both, then check n >= 2 keeps both.
	rows, err := repo.Aggregate(context.Background(), r3.Query{
		Filters:    r3.Filters{r3.Ne("name", "Berlin")},
		GroupBy:    r3.GroupBy("country"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.Gte("n", 2)},
	})
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	rows, err = repo.Aggregate(context.Background(), r3.Query{
		Filters:    r3.Filters{r3.Ne("name", "Berlin")},
		GroupBy:    r3.GroupBy("country"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.Gt("n", 2)},
	})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestAggregate_PaginationOverGroups(t *testing.T) {
	repo := seedAggregateCities(t)

	rows, err := repo.Aggregate(context.Background(), r3.Query{
		GroupBy:    r3.GroupBy("country"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("n"))},
		Pagination: r3.NewPaginationSpecWithSize(1),
	})
	require.NoError(t, err)
	require.Len(t, rows, 1, "pagination limits grouped rows")
	country, _ := rows[0].String("country")
	assert.Equal(t, "Germany", country)
}

func TestAggregate_SoftDeleteExcluded(t *testing.T) {
	repo := newJSONRepo[CityWithSoftDelete, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	a, err := repo.Create(ctx, CityWithSoftDelete{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, CityWithSoftDelete{Name: "Munich", Country: "Germany"})
	require.NoError(t, err)
	require.NoError(t, repo.Delete(ctx, a.ID))

	q := r3.Query{
		GroupBy:    r3.GroupBy("country"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	}

	rows, err := repo.Aggregate(ctx, q)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	n, _ := rows[0].Int64("n")
	assert.Equal(t, int64(1), n, "soft-deleted rows are excluded")

	trashed := q.Clone()
	trashed.IncludeTrashed = maybe.True()
	rows, err = repo.Aggregate(ctx, trashed)
	require.NoError(t, err)
	n, _ = rows[0].Int64("n")
	assert.Equal(t, int64(2), n, "IncludeTrashed counts soft-deleted rows too")
}

func TestAggregate_NullGroupKeyAndSkippedNulls(t *testing.T) {
	repo := newJSONRepo[Scored, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	five := "5"
	seven := "7"
	for _, s := range []Scored{
		{Score: 1, Optional: &five},
		{Score: 2, Optional: &seven},
		{Score: 3, Optional: nil},
	} {
		_, err := repo.Create(ctx, s)
		require.NoError(t, err)
	}

	// MIN/MAX/COUNT over a nullable field skip NULLs (SQL semantics).
	rows, err := repo.Aggregate(ctx, r3.Query{
		Aggregates: r3.Aggregates{
			r3.AggCount("total"),
			{Func: r3.AggregateCount, Field: r3.NewFieldSpec("optional"), Alias: "with_value"},
			r3.AggMin("optional", "min_opt"),
		},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)

	total, _ := rows[0].Int64("total")
	withValue, _ := rows[0].Int64("with_value")
	minOpt, _ := rows[0].Int64("min_opt")
	assert.Equal(t, int64(3), total)
	assert.Equal(t, int64(2), withValue, "COUNT(field) skips NULLs")
	assert.Equal(t, int64(5), minOpt)

	// Grouping by a nullable field: the nil bucket appears with a nil key.
	rows, err = repo.Aggregate(ctx, r3.Query{
		GroupBy:    r3.GroupBy("optional"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestAggregate_InvalidQuery(t *testing.T) {
	repo := seedAggregateCities(t)

	_, err := repo.Aggregate(context.Background(), r3.Query{
		GroupBy: r3.GroupBy("country"),
	})
	assert.ErrorIs(t, err, r3.ErrInvalidAggregate, "empty Aggregates is invalid")
}
