package r3gorm_test

import (
	"context"
	"testing"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// R3-013: negated relationship filters (anti-join / NOT EXISTS) via r3.HasNo.

func TestHasNo_ManyToMany(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	// Activists in NO squad among {1,3} → Cyd only (Ann∈{1}, Bob∈{1,3} excluded).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.HasNo("Squads", r3.In("id", []int64{1, 3}))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(a relActivist) string { return a.Name }); !sameSet(g, []string{"Cyd"}) {
		t.Fatalf("got %v, want [Cyd]", g)
	}
}

func TestHasNo_ManyToMany_EmptyKeySetMatchesAll(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relActivist, int64](db)
	ctx := context.Background()

	// Nobody is in squad 99, so every activist "has no squad in {99}" → all three.
	// This exercises the NOT IN () → TRUE collapse.
	got, total, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.HasNo("Squads", r3.In("id", []int64{99}))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if g := names(got, func(a relActivist) string { return a.Name }); !sameSet(g, []string{"Ann", "Bob", "Cyd"}) {
		t.Fatalf("got %v, want [Ann Bob Cyd]", g)
	}
}

func TestHasNo_HasMany_NoRelatedRows(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relSquad, int64](db)
	ctx := context.Background()

	// Squads with no members at all → Charlie (Alpha has Mara, Bravo has Nico).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.HasNo("Members")},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(s relSquad) string { return s.Name }); !sameSet(g, []string{"Charlie"}) {
		t.Fatalf("got %v, want [Charlie]", g)
	}
}

func TestHasNo_HasMany_WithInnerFilter(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relSquad, int64](db)
	ctx := context.Background()

	// Squads without a member named "Mara" → Bravo, Charlie (only Alpha has Mara).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.HasNo("Members", r3.Eq("name", "Mara"))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(s relSquad) string { return s.Name }); !sameSet(g, []string{"Bravo", "Charlie"}) {
		t.Fatalf("got %v, want [Bravo Charlie]", g)
	}
}

func TestHasNo_BelongsTo_IncludesNullFK(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relRaid, int64](db)
	ctx := context.Background()

	// Raids whose leader is NOT "Bob" — including the raid with NO leader (nil FK),
	// which the plain NOT IN would wrongly drop. R1 (Bob) excluded; R2 (Cyd) and
	// R3 (no leader) included.
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.HasNo("Leader", r3.Eq("name", "Bob"))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(r relRaid) string { return r.Title }); !sameSet(g, []string{"R2", "R3"}) {
		t.Fatalf("got %v, want [R2 R3]", g)
	}
}

func TestHasNo_BelongsTo_NoRelatedAtAll(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	repo := r3gorm.NewGormCRUD[relRaid, int64](db)
	ctx := context.Background()

	// Raids with no leader at all → R3 (nil leader_id).
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.HasNo("Leader")},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(r relRaid) string { return r.Title }); !sameSet(g, []string{"R3"}) {
		t.Fatalf("got %v, want [R3]", g)
	}
}

// R3-014: relations declared physically (table + columns) with NO Go struct
// field on the entity, resolved via r3.WithRelations.

// declActivist has NO Squads field — the m2m relation is declared explicitly.
type declActivist struct {
	ID   int64 `gorm:"primarykey"`
	Name string
}

func (declActivist) TableName() string { return "rel_activists" }

// declSquad has NO Members field.
type declSquad struct {
	ID   int64 `gorm:"primarykey"`
	Name string
}

func (declSquad) TableName() string { return "rel_squads" }

// declRaid has NO Leader field.
type declRaid struct {
	ID       int64 `gorm:"primarykey"`
	Title    string
	LeaderID *int64
}

func (declRaid) TableName() string { return "rel_raids" }

func TestDeclaredRelation_ManyToMany_NoStructField(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	ctx := context.Background()

	repo := r3gorm.NewGormCRUD[declActivist, int64](db, r3.WithRelations(
		r3.ManyToManyRelation("squads", "rel_activist_squads", "activist_id", "squad_id", "rel_squads"),
	))

	// Has resolves the declared relation exactly like a tag-declared one.
	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("squads", r3.In("id", []int64{1, 3}))},
	})
	if err != nil {
		t.Fatalf("has list: %v", err)
	}
	if g := names(got, func(a declActivist) string { return a.Name }); !sameSet(g, []string{"Ann", "Bob"}) {
		t.Fatalf("has got %v, want [Ann Bob]", g)
	}

	// And HasNo works through the same declared relation.
	got, _, err = repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.HasNo("squads", r3.In("id", []int64{1, 3}))},
	})
	if err != nil {
		t.Fatalf("hasno list: %v", err)
	}
	if g := names(got, func(a declActivist) string { return a.Name }); !sameSet(g, []string{"Cyd"}) {
		t.Fatalf("hasno got %v, want [Cyd]", g)
	}
}

func TestDeclaredRelation_HasMany_NoStructField(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	ctx := context.Background()

	repo := r3gorm.NewGormCRUD[declSquad, int64](db, r3.WithRelations(
		r3.HasManyRelation("members", "rel_members", "squad_id"),
	))

	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("members", r3.Eq("name", "Mara"))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(s declSquad) string { return s.Name }); !sameSet(g, []string{"Alpha"}) {
		t.Fatalf("got %v, want [Alpha]", g)
	}
}

func TestDeclaredRelation_BelongsTo_NoStructField(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	ctx := context.Background()

	repo := r3gorm.NewGormCRUD[declRaid, int64](db, r3.WithRelations(
		r3.BelongsToRelation("leader", "rel_activists", "leader_id"),
	))

	got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("leader", r3.Eq("name", "Bob"))},
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if g := names(got, func(r declRaid) string { return r.Title }); !sameSet(g, []string{"R1"}) {
		t.Fatalf("got %v, want [R1]", g)
	}
}
