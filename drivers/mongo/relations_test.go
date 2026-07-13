package r3mongo_test

import (
	"testing"

	"github.com/amberpixels/r3"
	r3mongo "github.com/amberpixels/r3/drivers/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Author struct {
	ID   bson.ObjectID `bson:"_id"`
	Name string        `bson:"name"`
}

type Book struct {
	ID       bson.ObjectID `bson:"_id"`
	Title    string        `bson:"title"`
	AuthorID bson.ObjectID `bson:"author_id"`
	Genre    string        `bson:"genre"`
}

// Event is joined to Tag via a many-to-many join collection ("event_tags").
type Event struct {
	ID   bson.ObjectID `bson:"_id"`
	Name string        `bson:"name"`
}

type Tag struct {
	ID    bson.ObjectID `bson:"_id"`
	Label string        `bson:"label"`
}

func TestMongoRelations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if !isDockerAvailable() {
		t.Skip("Docker not available - Mongo integration test requires Docker")
	}

	ctx := t.Context()
	db, cleanup, err := setupMongoContainer(ctx)
	if err != nil {
		t.Skipf("Failed to set up Mongo container: %v", err)
	}
	defer cleanup()

	authorRepo := r3mongo.NewMongoCRUD[Author, bson.ObjectID](db.Collection("authors"),
		r3.WithRelations(r3.HasManyRelation("books", "books", "author_id")))
	bookRepo := r3mongo.NewMongoCRUD[Book, bson.ObjectID](db.Collection("books"),
		r3.WithRelations(r3.BelongsToRelation("author", "authors", "author_id")))

	authors := map[string]Author{}
	for _, name := range []string{"Asimov", "Tolkien", "Silent"} {
		got, err := authorRepo.Create(ctx, Author{Name: name})
		require.NoError(t, err)
		authors[name] = got
	}
	// Asimov: scifi + fantasy. Tolkien: fantasy. Silent: none.
	for _, b := range []Book{
		{Title: "Foundation", AuthorID: authors["Asimov"].ID, Genre: "scifi"},
		{Title: "Gods Themselves", AuthorID: authors["Asimov"].ID, Genre: "fantasy"},
		{Title: "The Hobbit", AuthorID: authors["Tolkien"].ID, Genre: "fantasy"},
	} {
		_, err := bookRepo.Create(ctx, b)
		require.NoError(t, err)
	}

	names := func(as []Author) []string {
		out := make([]string, len(as))
		for i, a := range as {
			out[i] = a.Name
		}
		return out
	}

	t.Run("Has has-many with an inner filter", func(t *testing.T) {
		got, _, err := authorRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Has("books", r3.Eq("genre", "scifi"))},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Asimov"}, names(got))
	})

	t.Run("Has has-many with no inner filter (any child)", func(t *testing.T) {
		got, _, err := authorRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Has("books")},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Asimov", "Tolkien"}, names(got))
	})

	t.Run("HasNo has-many (anti-join)", func(t *testing.T) {
		got, _, err := authorRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.HasNo("books")},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Silent"}, names(got))
	})

	t.Run("HasNo has-many with an inner filter", func(t *testing.T) {
		// Authors with NO scifi book: Tolkien (fantasy only) and Silent (none).
		got, _, err := authorRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.HasNo("books", r3.Eq("genre", "scifi"))},
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Tolkien", "Silent"}, names(got))
	})

	t.Run("Count honors relationship filters", func(t *testing.T) {
		n, err := authorRepo.Count(ctx, r3.Query{Filters: r3.Filters{r3.Has("books")}})
		require.NoError(t, err)
		assert.Equal(t, int64(2), n)
	})

	t.Run("Has belongs-to", func(t *testing.T) {
		got, _, err := bookRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Has("author", r3.Eq("name", "Asimov"))},
		})
		require.NoError(t, err)
		assert.Len(t, got, 2, "both Asimov books")
	})

	t.Run("HasNo belongs-to includes a null foreign key", func(t *testing.T) {
		// A pointer FK lets "no author" store as null; the anti-join must include it.
		type Note struct {
			ID       bson.ObjectID  `bson:"_id"`
			Body     string         `bson:"body"`
			AuthorID *bson.ObjectID `bson:"author_id"`
		}
		noteRepo := r3mongo.NewMongoCRUD[Note, bson.ObjectID](db.Collection("notes"),
			r3.WithRelations(r3.BelongsToRelation("author", "authors", "author_id")))

		asimov := authors["Asimov"].ID
		_, err := noteRepo.Create(ctx, Note{Body: "by asimov", AuthorID: &asimov})
		require.NoError(t, err)
		_, err = noteRepo.Create(ctx, Note{Body: "orphan", AuthorID: nil})
		require.NoError(t, err)

		// HasNo author matching "Asimov": the orphan (null FK) AND notes by others.
		// Here only the orphan qualifies (the other note is by Asimov).
		got, _, err := noteRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.HasNo("author", r3.Eq("name", "Asimov"))},
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "orphan", got[0].Body, "a null FK must be included in the anti-join")
	})

	t.Run("unknown relation errors", func(t *testing.T) {
		_, _, err := authorRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Has("publishers")},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown relation")
	})

	t.Run("AggregateThroughRelation has-many grouped by genre", func(t *testing.T) {
		rows, err := authorRepo.AggregateThroughRelation(ctx, "books", r3.Query{
			GroupBy:    r3.GroupBy("genre"),
			Aggregates: r3.Aggregates{r3.AggCount("n")},
			Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("genre"))},
		})
		require.NoError(t, err)
		require.Len(t, rows, 2)
		byGenre := map[string]int64{}
		for _, row := range rows {
			g, _ := row.String("genre")
			n, _ := row.Int64("n")
			byGenre[g] = n
		}
		assert.Equal(t, int64(2), byGenre["fantasy"], "Gods Themselves + The Hobbit")
		assert.Equal(t, int64(1), byGenre["scifi"], "Foundation")
	})

	t.Run("AggregateThroughRelation restricted by owner filter", func(t *testing.T) {
		rows, err := authorRepo.AggregateThroughRelation(ctx, "books", r3.Query{
			Filters:    r3.Filters{r3.Eq("name", "Asimov")}, // owner filter
			Aggregates: r3.Aggregates{r3.AggCount("n")},
		})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		n, _ := rows[0].Int64("n")
		assert.Equal(t, int64(2), n, "only Asimov's two books")
	})

	t.Run("many-to-many Has and aggregate", func(t *testing.T) {
		eventRepo := r3mongo.NewMongoCRUD[Event, bson.ObjectID](db.Collection("events"),
			r3.WithRelations(r3.ManyToManyRelation("tags", "event_tags", "event_id", "tag_id", "tags")))
		tagRepo := r3mongo.NewMongoCRUD[Tag, bson.ObjectID](db.Collection("tags"))

		concert, err := eventRepo.Create(ctx, Event{Name: "Concert"})
		require.NoError(t, err)
		_, err = eventRepo.Create(ctx, Event{Name: "Talk"}) // no tags: must be excluded
		require.NoError(t, err)

		music, err := tagRepo.Create(ctx, Tag{Label: "music"})
		require.NoError(t, err)
		live, err := tagRepo.Create(ctx, Tag{Label: "live"})
		require.NoError(t, err)

		// Concert: music + live. Talk: (none).
		join := db.Collection("event_tags")
		_, err = join.InsertMany(ctx, []any{
			bson.D{{Key: "event_id", Value: concert.ID}, {Key: "tag_id", Value: music.ID}},
			bson.D{{Key: "event_id", Value: concert.ID}, {Key: "tag_id", Value: live.ID}},
		})
		require.NoError(t, err)

		got, _, err := eventRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Has("tags", r3.Eq("label", "music"))},
		})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "Concert", got[0].Name)

		anyTag, _, err := eventRepo.List(ctx, r3.Query{
			Filters: r3.Filters{r3.Has("tags")},
		})
		require.NoError(t, err)
		require.Len(t, anyTag, 1, "only Concert has any tag (Talk has none)")
		assert.Equal(t, "Concert", anyTag[0].Name)

		// Aggregate: count of tag links per event, restricted to Concert.
		rows, err := eventRepo.AggregateThroughRelation(ctx, "tags", r3.Query{
			Filters:    r3.Filters{r3.Eq("name", "Concert")},
			Aggregates: r3.Aggregates{r3.AggCount("n")},
		})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		n, _ := rows[0].Int64("n")
		assert.Equal(t, int64(2), n, "Concert has two tag links")
	})
}
