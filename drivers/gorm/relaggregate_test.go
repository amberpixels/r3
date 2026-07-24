package r3gorm_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	"github.com/amberpixels/r3/features/permissions"
)

// R3-011: aggregation through a relation (COUNT/etc over a join or child table).

type aggGroup struct {
	ID     int64 `gorm:"primarykey"`
	Name   string
	Region string
}

func (aggGroup) TableName() string { return "agg_groups" }

type aggPerson struct {
	ID        int64 `gorm:"primarykey"`
	Name      string
	DeletedAt gorm.DeletedAt
}

func (aggPerson) TableName() string { return "agg_people" }

// setupRelAggDB builds a group/person schema with a soft-deletable person table and
// a group_people join, then seeds it. Membership (by person id):
//
//	group 1 (north): {1, 2, 4}   group 2 (south): {3}   group 3 (north): {}
//
// Person 4 (Dan) is then soft-deleted, so active membership is {1,2}, {3}, {}.
func setupRelAggDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&aggGroup{}, &aggPerson{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(
		`CREATE TABLE agg_group_people (group_id INTEGER NOT NULL, person_id INTEGER NOT NULL,
		 PRIMARY KEY (group_id, person_id))`,
	).Error; err != nil {
		t.Fatalf("create join: %v", err)
	}

	groups := []aggGroup{
		{ID: 1, Name: "Alpha", Region: "north"},
		{ID: 2, Name: "Bravo", Region: "south"},
		{ID: 3, Name: "Charlie", Region: "north"},
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatalf("seed groups: %v", err)
	}
	people := []aggPerson{{ID: 1, Name: "Ann"}, {ID: 2, Name: "Bob"}, {ID: 3, Name: "Cyd"}, {ID: 4, Name: "Dan"}}
	if err := db.Create(&people).Error; err != nil {
		t.Fatalf("seed people: %v", err)
	}
	joins := []struct{ GroupID, PersonID int64 }{{1, 1}, {1, 2}, {1, 4}, {2, 3}}
	for _, j := range joins {
		if err := db.Exec(
			"INSERT INTO agg_group_people (group_id, person_id) VALUES (?, ?)", j.GroupID, j.PersonID,
		).Error; err != nil {
			t.Fatalf("seed join: %v", err)
		}
	}
	// Soft-delete Dan (person 4): gorm stamps deleted_at.
	if err := db.Delete(&aggPerson{}, 4).Error; err != nil {
		t.Fatalf("soft-delete: %v", err)
	}
	return db
}

// countByGroup collapses aggregate rows keyed by "group_id" into a map.
func countByGroup(t *testing.T, rows []r3.AggregateRow) map[int64]int64 {
	t.Helper()
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		key, ok := row.Int64("group_id")
		if !ok {
			t.Fatalf("row missing group_id: %v", row)
		}
		n, ok := row.Int64("n")
		if !ok {
			t.Fatalf("row missing n: %v", row)
		}
		out[key] = n
	}
	return out
}

func TestAggregateThroughRelation_M2M_ExcludesSoftDeleted(t *testing.T) {
	db := setupRelAggDB(t)
	ctx := context.Background()

	repo := r3gorm.NewGormCRUD[aggGroup, int64](db, r3.WithRelations(
		r3.ManyToManyRelation("members", "agg_group_people", "group_id", "person_id", "agg_people",
			r3.RelationTargetSoftDelete("deleted_at")),
	))

	rows, err := r3.AggregateThroughRelation(ctx, repo, "members", r3.Query{
		GroupBy:    r3.GroupBy("group_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}

	got := countByGroup(t, rows)
	// group 1 = {1,2} (Dan soft-deleted, excluded), group 2 = {3}. group 3 has no
	// members, so no row.
	want := map[int64]int64{1: 2, 2: 1}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("group %d = %d, want %d (all: %v)", k, got[k], v, got)
		}
	}
}

func TestAggregateThroughRelation_M2M_WithoutSoftDeleteCountsRawJoinRows(t *testing.T) {
	db := setupRelAggDB(t)
	ctx := context.Background()

	// No RelationTargetSoftDelete: aggregation counts raw join rows, so the
	// soft-deleted person is still counted (group 1 = 3).
	repo := r3gorm.NewGormCRUD[aggGroup, int64](db, r3.WithRelations(
		r3.ManyToManyRelation("members_raw", "agg_group_people", "group_id", "person_id", "agg_people"),
	))

	rows, err := r3.AggregateThroughRelation(ctx, repo, "members_raw", r3.Query{
		GroupBy:    r3.GroupBy("group_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if got := countByGroup(t, rows); got[1] != 3 || got[2] != 1 {
		t.Fatalf("got %v, want group1=3 group2=1", got)
	}
}

func TestAggregateThroughRelation_OwnerFilterRestricts(t *testing.T) {
	db := setupRelAggDB(t)
	ctx := context.Background()

	repo := r3gorm.NewGormCRUD[aggGroup, int64](db, r3.WithRelations(
		r3.ManyToManyRelation("members", "agg_group_people", "group_id", "person_id", "agg_people",
			r3.RelationTargetSoftDelete("deleted_at")),
	))

	// Filters are owner (group) filters: only north-region groups {1,3}. Group 3
	// has no members, so only group 1 appears.
	rows, err := r3.AggregateThroughRelation(ctx, repo, "members", r3.Query{
		Filters:    r3.Filters{r3.Eq("region", "north")},
		GroupBy:    r3.GroupBy("group_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	got := countByGroup(t, rows)
	if len(got) != 1 || got[1] != 2 {
		t.Fatalf("got %v, want {1:2}", got)
	}
}

func TestAggregateThroughRelation_Having(t *testing.T) {
	db := setupRelAggDB(t)
	ctx := context.Background()

	repo := r3gorm.NewGormCRUD[aggGroup, int64](db, r3.WithRelations(
		r3.ManyToManyRelation("members", "agg_group_people", "group_id", "person_id", "agg_people",
			r3.RelationTargetSoftDelete("deleted_at")),
	))

	// Only groups with >= 2 active members → group 1.
	rows, err := r3.AggregateThroughRelation(ctx, repo, "members", r3.Query{
		GroupBy:    r3.GroupBy("group_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
		Having:     r3.Filters{r3.Gte("n", int64(2))},
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	got := countByGroup(t, rows)
	if len(got) != 1 || got[1] != 2 {
		t.Fatalf("got %v, want {1:2}", got)
	}
}

func TestAggregateThroughRelation_HasMany_ReflectedRelation(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	ctx := context.Background()

	// Uses the tag-reflected has-many "Members" on relSquad (child table rel_members).
	repo := r3gorm.NewGormCRUD[relSquad, int64](db)

	rows, err := r3.AggregateThroughRelation(ctx, repo, "Members", r3.Query{
		GroupBy:    r3.GroupBy("squad_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	// Mara ∈ squad 1, Nico ∈ squad 2; squad 3 has no members.
	out := make(map[int64]int64)
	for _, row := range rows {
		k, _ := row.Int64("squad_id")
		n, _ := row.Int64("n")
		out[k] = n
	}
	if out[1] != 1 || out[2] != 1 || len(out) != 2 {
		t.Fatalf("got %v, want {1:1, 2:1}", out)
	}
}

func TestAggregateThroughRelation_BelongsToNotAggregatable(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	ctx := context.Background()

	repo := r3gorm.NewGormCRUD[declRaid, int64](db, r3.WithRelations(
		r3.BelongsToRelation("leader", "rel_activists", "leader_id"),
	))

	_, err := r3.AggregateThroughRelation(ctx, repo, "leader", r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err == nil {
		t.Fatalf("expected error aggregating a belongs-to relation, got nil")
	}
}

func TestAggregateThroughRelation_UnknownRelation(t *testing.T) {
	db := setupRelAggDB(t)
	ctx := context.Background()
	repo := r3gorm.NewGormCRUD[aggGroup, int64](db)

	_, err := r3.AggregateThroughRelation(ctx, repo, "nope", r3.Query{
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err == nil {
		t.Fatalf("expected error for unknown relation, got nil")
	}
}

func TestAggregateThroughRelation_NotSupportedByBackend(t *testing.T) {
	// A repo value that is not a RelationAggregator yields the sentinel.
	var repo r3.Querier[aggGroup, int64] = notARelationAggregator[aggGroup, int64]{}
	_, err := r3.AggregateThroughRelation(context.Background(), repo, "x")
	if !errors.Is(err, r3.ErrRelationAggregateNotSupported) {
		t.Fatalf("got %v, want ErrRelationAggregateNotSupported", err)
	}
}

// groupRegionScope scopes aggGroup rows to the region named in the actor claims;
// no claim sees everything.
type groupRegionScope struct{}

func (groupRegionScope) Check(context.Context, permissions.AccessRequest[aggGroup, int64]) error {
	return nil
}

func (groupRegionScope) Scope(_ context.Context, actor r3.Actor) (r3.Filters, error) {
	region, ok := actor.Claims.(string)
	if !ok {
		return nil, nil
	}
	return r3.Filters{r3.Eq("region", region)}, nil
}

// TestAggregateThroughRelation_PermissionsScoped is the load-bearing case:
// relation aggregation must route through the decorator stack, so a scoped
// actor's aggregate only folds related rows of owners they may see — otherwise
// it would be a side door around row visibility.
func TestAggregateThroughRelation_PermissionsScoped(t *testing.T) {
	db := setupRelAggDB(t)
	// Actor scoped to the "north" region → groups {1,3}.
	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "u1", Type: "user", Claims: "north"})

	base := r3gorm.NewGormCRUD[aggGroup, int64](db, r3.WithRelations(
		r3.ManyToManyRelation("members", "agg_group_people", "group_id", "person_id", "agg_people",
			r3.RelationTargetSoftDelete("deleted_at")),
	))
	repo := permissions.WithPermissions[aggGroup, int64](base, groupRegionScope{})

	rows, err := r3.AggregateThroughRelation(ctx, repo, "members", r3.Query{
		GroupBy:    r3.GroupBy("group_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	// north groups {1,3}; group 3 has no members → only group 1 (2 active members).
	got := countByGroup(t, rows)
	if len(got) != 1 || got[1] != 2 {
		t.Fatalf("got %v, want {1:2} (scoped to north)", got)
	}
}

// notARelationAggregator implements Querier but not RelationAggregator.
type notARelationAggregator[T any, ID comparable] struct{}

func (notARelationAggregator[T, ID]) Get(context.Context, ID, ...r3.Query) (T, error) {
	var z T
	return z, nil
}

func (notARelationAggregator[T, ID]) List(context.Context, ...r3.Query) ([]T, int64, error) {
	return nil, 0, nil
}

func (notARelationAggregator[T, ID]) Count(context.Context, ...r3.Query) (int64, error) {
	return 0, nil
}
