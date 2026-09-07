package r3gorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// aggRaid mirrors the shape that motivated the aggregation API (p44's raid
// stats): a nullable FK key, a date, and gorm soft-delete.
type aggRaid struct {
	ID         int64 `gorm:"primarykey"`
	LocationID *int64
	SquadID    *int64
	Date       time.Time
	DeletedAt  gorm.DeletedAt
}

func (aggRaid) TableName() string { return "agg_raids" }

func setupAggDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(&aggRaid{}))
	return db
}

func day(d int) time.Time {
	return time.Date(2026, 1, d, 12, 0, 0, 0, time.UTC)
}

func TestGormAggregate_RaidStatsByLocation(t *testing.T) {
	db := setupAggDB(t)
	repo := r3gorm.NewGormCRUD[aggRaid, int64](db)
	ctx := context.Background()

	seed := []aggRaid{
		{LocationID: new(int64(1)), SquadID: new(int64(10)), Date: day(1)},
		{LocationID: new(int64(1)), SquadID: new(int64(10)), Date: day(5)},
		{LocationID: new(int64(1)), SquadID: new(int64(20)), Date: day(3)},
		{LocationID: new(int64(2)), SquadID: new(int64(10)), Date: day(2)},
		{LocationID: nil, SquadID: new(int64(20)), Date: day(4)}, // no location — excluded by filter
	}
	for _, raid := range seed {
		_, err := repo.Create(ctx, raid)
		require.NoError(t, err)
	}
	// A soft-deleted raid must not count.
	deleted, err := repo.Create(ctx, aggRaid{LocationID: new(int64(1)), SquadID: new(int64(10)), Date: day(9)})
	require.NoError(t, err)
	require.NoError(t, repo.Delete(ctx, deleted.ID))

	// The exact query p44's RaidStatsReader.ByLocation hand-writes in SQL.
	rows, err := repo.Aggregate(ctx, r3.Query{
		Filters: r3.Filters{r3.Ne("location_id", nil)},
		GroupBy: r3.GroupBy("location_id", "squad_id"),
		Aggregates: r3.Aggregates{
			r3.AggCount("raids"),
			r3.AggMax("date", "last_date"),
			r3.AggMin("date", "first_date"),
		},
		Sorts: r3.Sorts{
			r3.NewSortAscSpec(r3.NewFieldSpec("location_id")),
			r3.NewSortAscSpec(r3.NewFieldSpec("first_date")),
		},
	})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	// location 1 / squad 10: 2 raids (the soft-deleted third is excluded).
	loc, _ := rows[0].Int64("location_id")
	squad, _ := rows[0].Int64("squad_id")
	n, _ := rows[0].Int64("raids")
	assert.Equal(t, int64(1), loc)
	assert.Equal(t, int64(10), squad)
	assert.Equal(t, int64(2), n)

	last, ok := rows[0].Time("last_date")
	require.True(t, ok, "Time() must parse gorm/sqlite MAX(date), got %#v", rows[0].Value("last_date"))
	assert.Equal(t, 5, last.Day())
	first, _ := rows[0].Time("first_date")
	assert.Equal(t, 1, first.Day())

	// location 1 / squad 20 sorts after squad 10 (first raid day 3 > day 1).
	squad, _ = rows[1].Int64("squad_id")
	assert.Equal(t, int64(20), squad)

	loc, _ = rows[2].Int64("location_id")
	assert.Equal(t, int64(2), loc)

	// The IN restriction (list-enrichment case).
	rows, err = repo.Aggregate(ctx, r3.Query{
		Filters: r3.Filters{
			r3.Ne("location_id", nil),
			r3.In("location_id", []int64{2}),
		},
		GroupBy:    r3.GroupBy("location_id", "squad_id"),
		Aggregates: r3.Aggregates{r3.AggCount("raids")},
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	n, _ = rows[0].Int64("raids")
	assert.Equal(t, int64(1), n)
}

func TestGormAggregate_GroupCountPerSquad(t *testing.T) {
	db := setupAggDB(t)
	repo := r3gorm.NewGormCRUD[aggRaid, int64](db)
	ctx := context.Background()

	for _, raid := range []aggRaid{
		{SquadID: new(int64(10)), Date: day(1)},
		{SquadID: new(int64(10)), Date: day(2)},
		{SquadID: new(int64(20)), Date: day(3)},
		{SquadID: nil, Date: day(4)}, // NULL squad — excluded, like p44's groupCount
	} {
		_, err := repo.Create(ctx, raid)
		require.NoError(t, err)
	}

	// p44's svcsquadstats.groupCount: squad_id IS NOT NULL, GROUP BY squad_id.
	rows, err := repo.Aggregate(ctx, r3.Query{
		Filters:    r3.Filters{r3.Ne("squad_id", nil)},
		GroupBy:    r3.GroupBy("squad_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n"), r3.AggMax("date", "last")},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("squad_id"))},
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)

	counts := map[int64]int64{}
	for _, row := range rows {
		squad, _ := row.Int64("squad_id")
		n, _ := row.Int64("n")
		counts[squad] = n
	}
	assert.Equal(t, map[int64]int64{10: 2, 20: 1}, counts)
}

func TestGormAggregate_DefaultSortIsDropped(t *testing.T) {
	db := setupAggDB(t)
	repo := r3gorm.NewGormCRUD[aggRaid, int64](db)
	ctx := context.Background()

	// A repo-level default sort on an entity column must not leak into the
	// grouped query (it would be invalid SQL under GROUP BY).
	repo.SetDefaultListQuery(r3.Query{
		Sorts: r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("date"))},
	})

	_, err := repo.Create(ctx, aggRaid{SquadID: new(int64(10)), Date: day(1)})
	require.NoError(t, err)

	rows, err := repo.Aggregate(ctx, r3.Query{
		GroupBy:    r3.GroupBy("squad_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	require.NoError(t, err, "inherited default sort must be dropped, not break the query")
	assert.Len(t, rows, 1)
}
