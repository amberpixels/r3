package r3gorm_test

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// Test models exercising all three relation kinds the lowering supports.

type relSquad struct {
	ID      int64 `gorm:"primarykey"`
	Name    string
	Members []relMember `gorm:"-"          r3:"rel:has-many,fk:squad_id"`
}

func (relSquad) TableName() string { return "rel_squads" }

type relMember struct {
	ID      int64 `gorm:"primarykey"`
	Name    string
	SquadID int64
}

func (relMember) TableName() string { return "rel_members" }

type relActivist struct {
	ID     int64 `gorm:"primarykey"`
	Name   string
	Squads []relSquad `gorm:"-"          r3:"rel:many-to-many,join:rel_activist_squads,fk:activist_id,ref:squad_id"`
}

func (relActivist) TableName() string { return "rel_activists" }

type relRaid struct {
	ID       int64 `gorm:"primarykey"`
	Title    string
	LeaderID *int64
	Leader   *relActivist `gorm:"-"          r3:"rel:belongs-to,fk:leader_id"`
}

func (relRaid) TableName() string { return "rel_raids" }

func setupRelDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Keep a single connection so the in-memory DB persists across queries.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&relSquad{}, &relMember{}, &relActivist{}, &relRaid{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(
		`CREATE TABLE rel_activist_squads (activist_id INTEGER NOT NULL, squad_id INTEGER NOT NULL,
		 PRIMARY KEY (activist_id, squad_id))`,
	).Error; err != nil {
		t.Fatalf("create join: %v", err)
	}
	return db
}

func seedRelData(t *testing.T, db *gorm.DB) {
	t.Helper()

	squads := []relSquad{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Bravo"}, {ID: 3, Name: "Charlie"}}
	if err := db.Create(&squads).Error; err != nil {
		t.Fatalf("seed squads: %v", err)
	}

	activists := []relActivist{{ID: 1, Name: "Ann"}, {ID: 2, Name: "Bob"}, {ID: 3, Name: "Cyd"}}
	if err := db.Create(&activists).Error; err != nil {
		t.Fatalf("seed activists: %v", err)
	}

	// Ann ∈ {1}, Bob ∈ {1,3}, Cyd ∈ {2}.
	joins := []struct {
		ActivistID int64
		SquadID    int64
	}{{1, 1}, {2, 1}, {2, 3}, {3, 2}}
	for _, j := range joins {
		if err := db.Exec(
			"INSERT INTO rel_activist_squads (activist_id, squad_id) VALUES (?, ?)", j.ActivistID, j.SquadID,
		).Error; err != nil {
			t.Fatalf("seed join: %v", err)
		}
	}

	// Members: Alpha has "Mara", Bravo has "Nico".
	members := []relMember{{ID: 1, Name: "Mara", SquadID: 1}, {ID: 2, Name: "Nico", SquadID: 2}}
	if err := db.Create(&members).Error; err != nil {
		t.Fatalf("seed members: %v", err)
	}

	leader2 := int64(2)
	leader3 := int64(3)
	raids := []relRaid{
		{ID: 1, Title: "R1", LeaderID: &leader2}, // led by Bob
		{ID: 2, Title: "R2", LeaderID: &leader3}, // led by Cyd
		{ID: 3, Title: "R3", LeaderID: nil},      // no leader
	}
	if err := db.Create(&raids).Error; err != nil {
		t.Fatalf("seed raids: %v", err)
	}
}

func names[T any](items []T, f func(T) string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = f(it)
	}
	return out
}

func TestRelationFilter_ManyToMany(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	// Activists belonging to squad 1 or 3 → Ann (1), Bob (1,3). Cyd (2) excluded.
	got, total, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("Squads", r3.In("id", []int64{1, 3}))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if g := names(got, func(a relActivist) string { return a.Name }); !sameSet(
		g,
		[]string{"Ann", "Bob"},
	) {
		t.Fatalf("got %v, want [Ann Bob]", g)
	}
}

func TestRelationFilter_ManyToMany_InnerOnTargetColumn(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	// Filter by the related squad's NAME, not its id (the two-hop path).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("Squads", r3.Eq("name", "Charlie"))}, // squad 3
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(a relActivist) string { return a.Name }); !sameSet(g, []string{"Bob"}) {
		t.Fatalf("got %v, want [Bob]", g)
	}
}

func TestRelationFilter_EmptyResult(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	// No squad 99 → no activists. Must be empty, not a SQL error from "IN ()".
	got, total, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("Squads", r3.In("id", []int64{99}))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(got) != 0 {
		t.Fatalf("got %d rows (total %d), want 0", len(got), total)
	}
}

func TestRelationFilter_BelongsTo(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relRaid, int64](db)
	ctx := context.Background()

	// Raids led by someone named "Bob" → R1 (leader_id 2).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("Leader", r3.Eq("name", "Bob"))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(r relRaid) string { return r.Title }); !sameSet(g, []string{"R1"}) {
		t.Fatalf("got %v, want [R1]", g)
	}
}

func TestRelationFilter_HasMany(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relSquad, int64](db)
	ctx := context.Background()

	// Squads that have a member named "Mara" → Alpha (squad 1).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("Members", r3.Eq("name", "Mara"))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(s relSquad) string { return s.Name }); !sameSet(g, []string{"Alpha"}) {
		t.Fatalf("got %v, want [Alpha]", g)
	}
}

func TestRelationFilter_ComposedWithColumnFilter(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	// In squad 1 AND name = "Ann" → Ann only (Bob is also in squad 1).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{
			r3.Has("Squads", r3.In("id", []int64{1})),
			r3.Eq("name", "Ann"),
		},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(a relActivist) string { return a.Name }); !sameSet(g, []string{"Ann"}) {
		t.Fatalf("got %v, want [Ann]", g)
	}
}

func TestRelationFilter_Count(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	total, err := repo.Count(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("Squads", r3.In("id", []int64{1, 3}))},
	})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 2 {
		t.Fatalf("count = %d, want 2", total)
	}
}

func TestRelationFilter_UnknownRelation(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	_, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("Nope", r3.In("id", []int64{1}))},
	})
	if err == nil {
		t.Fatalf("expected error for unknown relation, got nil")
	}
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]int, len(got))
	for _, g := range got {
		seen[g]++
	}
	for _, w := range want {
		if seen[w] == 0 {
			return false
		}
		seen[w]--
	}
	return true
}
