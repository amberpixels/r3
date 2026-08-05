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

// Models exercising the qualified M2M predicate (`where:relation=…`): one join
// table with a discriminator column backs two relations, an article's curated
// topics and the tags it merely mentions.

type qualTag struct {
	ID    int64 `gorm:"primarykey"`
	Label string
}

func (qualTag) TableName() string { return "qual_tags" }

type qualArticle struct {
	ID    int64 `gorm:"primarykey"`
	Title string
	//nolint:lll // the relation tags are one clause per concern and do not read better wrapped
	Topics []qualTag `gorm:"-" r3:"rel:many-to-many,join:qual_article_tags,fk:article_id,ref:tag_id,order:sort_order,where:relation=topic"`
	//nolint:lll // same
	Mentions []qualTag `gorm:"-" r3:"rel:many-to-many,join:qual_article_tags,fk:article_id,ref:tag_id,where:relation=mention"`
}

func (qualArticle) TableName() string { return "qual_articles" }

// setupQualDB builds the article/tag schema with a single discriminated join
// table. `relation` defaults to 'topic', the shape an existing table takes when
// the column is added without a backfill: every pre-existing row already means
// the strong relation.
func setupQualDB(t *testing.T) *gorm.DB {
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

	if err := db.AutoMigrate(&qualTag{}, &qualArticle{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(
		`CREATE TABLE qual_article_tags (article_id INTEGER NOT NULL, tag_id INTEGER NOT NULL,
		 sort_order INTEGER NOT NULL DEFAULT 0, relation TEXT NOT NULL DEFAULT 'topic',
		 PRIMARY KEY (article_id, tag_id, relation))`,
	).Error; err != nil {
		t.Fatalf("create join: %v", err)
	}

	tags := []qualTag{{ID: 1, Label: "go"}, {ID: 2, Label: "sql"}, {ID: 3, Label: "mongo"}}
	if err := db.Create(&tags).Error; err != nil {
		t.Fatalf("seed tags: %v", err)
	}
	return db
}

func tagIDs(tags []qualTag) []int64 {
	ids := make([]int64, len(tags))
	for i, tag := range tags {
		ids[i] = tag.ID
	}
	return ids
}

var qualPreload = r3.Query{Preloads: r3.Preloads{
	r3.NewPreloadSpec("Topics"), r3.NewPreloadSpec("Mentions"),
}}

// TestM2MQualified_RoundTrip: two relations over one join table round-trip
// independently - each preloads only its own rows (in `order:` order where
// declared), and saving either leaves the other's rows untouched.
func TestM2MQualified_RoundTrip(t *testing.T) {
	db := setupQualDB(t)
	articles := r3gorm.NewGormCRUD[qualArticle, int64](db)
	ctx := context.Background()

	created, err := articles.Create(ctx, qualArticle{
		ID: 1, Title: "Relations",
		Topics:   []qualTag{{ID: 3}, {ID: 1}},
		Mentions: []qualTag{{ID: 2}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := articles.Get(ctx, created.ID, qualPreload)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if want := []int64{3, 1}; !equalIDs(tagIDs(got.Topics), want) {
		t.Fatalf("topics: got %v, want %v", tagIDs(got.Topics), want)
	}
	if want := []int64{2}; !equalIDs(tagIDs(got.Mentions), want) {
		t.Fatalf("mentions: got %v, want %v", tagIDs(got.Mentions), want)
	}

	// Rewrite the topics only: the mention must survive, since the relation's
	// DELETE is scoped to its own slice.
	got.Topics = []qualTag{{ID: 1}}
	if _, err := articles.Update(ctx, got); err != nil {
		t.Fatalf("update topics: %v", err)
	}

	got, err = articles.Get(ctx, created.ID, qualPreload)
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if want := []int64{1}; !equalIDs(tagIDs(got.Topics), want) {
		t.Fatalf("topics after update: got %v, want %v", tagIDs(got.Topics), want)
	}
	if want := []int64{2}; !equalIDs(tagIDs(got.Mentions), want) {
		t.Fatalf("mentions after topic update: got %v, want %v", tagIDs(got.Mentions), want)
	}

	// Clearing the mentions leaves the topic in place, the mirror case.
	got.Mentions = []qualTag{}
	if _, err := articles.Update(ctx, got); err != nil {
		t.Fatalf("clear mentions: %v", err)
	}
	got, err = articles.Get(ctx, created.ID, qualPreload)
	if err != nil {
		t.Fatalf("get after clearing mentions: %v", err)
	}
	if want := []int64{1}; !equalIDs(tagIDs(got.Topics), want) {
		t.Fatalf("topics after clearing mentions: got %v, want %v", tagIDs(got.Topics), want)
	}
	if len(got.Mentions) != 0 {
		t.Fatalf("mentions after clearing: got %v, want none", tagIDs(got.Mentions))
	}
}

// TestM2MQualified_WritesTheConstant: a row written through a qualified relation
// carries the constant, so the same relation finds it on read with no backfill.
func TestM2MQualified_WritesTheConstant(t *testing.T) {
	db := setupQualDB(t)
	articles := r3gorm.NewGormCRUD[qualArticle, int64](db)
	ctx := context.Background()

	if _, err := articles.Create(ctx, qualArticle{
		ID: 1, Title: "Relations",
		Topics:   []qualTag{{ID: 1}},
		Mentions: []qualTag{{ID: 1}},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// The same tag on both relations is two distinct join rows, told apart by the
	// discriminator alone.
	var rows []struct {
		TagID     int64
		Relation  string
		SortOrder int
	}
	if err := db.Raw(
		"SELECT tag_id, relation, sort_order FROM qual_article_tags WHERE article_id = 1 ORDER BY relation",
	).Scan(&rows).Error; err != nil {
		t.Fatalf("read join: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("join rows: got %d, want 2 (%v)", len(rows), rows)
	}
	if rows[0].Relation != "mention" || rows[1].Relation != "topic" {
		t.Fatalf("relation constants: got %q and %q", rows[0].Relation, rows[1].Relation)
	}
	// Only the topic relation declares `order:`; the mention row keeps the
	// column's default.
	if rows[1].SortOrder != 0 || rows[0].SortOrder != 0 {
		t.Fatalf("sort_order: got %d (mention) and %d (topic), want 0 and 0", rows[0].SortOrder, rows[1].SortOrder)
	}
}

// TestM2MQualified_RelationFilters: Has and HasNo resolve against the relation's
// own slice, so the two relations over one join table disagree.
func TestM2MQualified_RelationFilters(t *testing.T) {
	db := setupQualDB(t)
	articles := r3gorm.NewGormCRUD[qualArticle, int64](db)
	ctx := context.Background()

	// Article 1 is about go and mentions sql; article 2 is about sql.
	for _, a := range []qualArticle{
		{ID: 1, Title: "About go", Topics: []qualTag{{ID: 1}}, Mentions: []qualTag{{ID: 2}}},
		{ID: 2, Title: "About sql", Topics: []qualTag{{ID: 2}}},
	} {
		if _, err := articles.Create(ctx, a); err != nil {
			t.Fatalf("create %d: %v", a.ID, err)
		}
	}

	listIDs := func(t *testing.T, filter *r3.FilterSpec) []int64 {
		t.Helper()
		got, _, err := articles.List(ctx, r3.Query{Filters: r3.Filters{filter}})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		ids := make([]int64, len(got))
		for i, a := range got {
			ids[i] = a.ID
		}
		return ids
	}

	// "sql" is article 2's topic and article 1's mention: one query per relation,
	// two different answers.
	if got := listIDs(t, r3.Has("Topics", r3.Eq("label", "sql"))); !equalIDs(got, []int64{2}) {
		t.Fatalf("Has(Topics, sql): got %v, want [2]", got)
	}
	if got := listIDs(t, r3.Has("Mentions", r3.Eq("label", "sql"))); !equalIDs(got, []int64{1}) {
		t.Fatalf("Has(Mentions, sql): got %v, want [1]", got)
	}
	// Article 2 has no mentions at all, so the anti-join keeps it.
	if got := listIDs(t, r3.HasNo("Mentions", r3.Eq("label", "sql"))); !equalIDs(got, []int64{2}) {
		t.Fatalf("HasNo(Mentions, sql): got %v, want [2]", got)
	}
}

// TestM2MQualified_RelationAggregate: aggregation folds the join table, so an
// unqualified fold would count both slices. Each relation counts only its own.
func TestM2MQualified_RelationAggregate(t *testing.T) {
	db := setupQualDB(t)
	articles := r3gorm.NewGormCRUD[qualArticle, int64](db)
	ctx := context.Background()

	if _, err := articles.Create(ctx, qualArticle{
		ID: 1, Title: "Relations",
		Topics:   []qualTag{{ID: 1}, {ID: 3}},
		Mentions: []qualTag{{ID: 2}},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	countFor := func(t *testing.T, relation string) int64 {
		t.Helper()
		rows, err := r3.AggregateThroughRelation(ctx, articles, relation, r3.Query{
			GroupBy:    r3.GroupBy("article_id"),
			Aggregates: r3.Aggregates{r3.AggCount("n")},
		})
		if err != nil {
			t.Fatalf("aggregate %s: %v", relation, err)
		}
		if len(rows) != 1 {
			t.Fatalf("aggregate %s: got %d rows, want 1", relation, len(rows))
		}
		n, ok := rows[0].Int64("n")
		if !ok {
			t.Fatalf("aggregate %s: row missing n: %v", relation, rows[0])
		}
		return n
	}

	if got := countFor(t, "Topics"); got != 2 {
		t.Fatalf("topic count: got %d, want 2", got)
	}
	if got := countFor(t, "Mentions"); got != 1 {
		t.Fatalf("mention count: got %d, want 1", got)
	}
}

// TestM2MQualified_DeclaredRelation: the same predicate declared physically via
// r3.RelationWhere, for an entity that does not import the related Go type.
func TestM2MQualified_DeclaredRelation(t *testing.T) {
	db := setupQualDB(t)
	ctx := context.Background()

	// Seed through the tag-declared relations, read back through declared ones.
	if _, err := r3gorm.NewGormCRUD[qualArticle, int64](db).Create(ctx, qualArticle{
		ID: 1, Title: "Relations",
		Topics:   []qualTag{{ID: 1}, {ID: 3}},
		Mentions: []qualTag{{ID: 2}},
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	repo := r3gorm.NewGormCRUD[qualArticle, int64](db, r3.WithRelations(
		r3.ManyToManyRelation("topics", "qual_article_tags", "article_id", "tag_id", "qual_tags",
			r3.RelationWhere("relation", "topic")),
		r3.ManyToManyRelation("mentions", "qual_article_tags", "article_id", "tag_id", "qual_tags",
			r3.RelationWhere("relation", "mention")),
	))

	got, _, err := repo.List(ctx, r3.Query{Filters: r3.Filters{r3.Has("mentions", r3.Eq("label", "sql"))}})
	if err != nil {
		t.Fatalf("list by declared relation: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("Has(mentions, sql): got %v, want article 1", got)
	}
	if got, _, err := repo.List(ctx, r3.Query{
		Filters: r3.Filters{r3.Has("topics", r3.Eq("label", "sql"))},
	}); err != nil {
		t.Fatalf("list by declared relation: %v", err)
	} else if len(got) != 0 {
		t.Fatalf("Has(topics, sql): got %v, want none", got)
	}

	rows, err := r3.AggregateThroughRelation(ctx, repo, "topics", r3.Query{
		GroupBy:    r3.GroupBy("article_id"),
		Aggregates: r3.Aggregates{r3.AggCount("n")},
	})
	if err != nil {
		t.Fatalf("aggregate declared relation: %v", err)
	}
	if n, ok := rows[0].Int64("n"); !ok || n != 2 {
		t.Fatalf("declared topic count: got %v, want 2", rows[0])
	}
}
