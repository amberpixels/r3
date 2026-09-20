package i18n_test

import (
	"context"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/i18n"
)

// Author is a pointer-shaped child: one translatable field and one the relation
// does not declare.
type Author struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
	Bio  string `db:"bio"`
}

// Tag is a slice-shaped child.
type Tag struct {
	ID    int64  `db:"id,pk"`
	Label string `db:"label"`
}

// Post is the parent: translatable itself, and preloads one Author and many Tags.
type Post struct {
	ID     int64  `db:"id,pk"`
	Title  string `db:"title"`
	Author *Author
	Tags   []Tag
}

// --- in-memory parent repo ---------------------------------------------

type memoryPostCRUD struct{ posts []Post }

func (m *memoryPostCRUD) Get(_ context.Context, id int64, _ ...r3.Query) (Post, error) {
	for _, p := range m.posts {
		if p.ID == id {
			return p, nil
		}
	}
	return Post{}, r3.ErrNotFound
}

func (m *memoryPostCRUD) List(_ context.Context, _ ...r3.Query) ([]Post, int64, error) {
	out := make([]Post, len(m.posts))
	copy(out, m.posts)
	return out, int64(len(out)), nil
}

func (m *memoryPostCRUD) Count(_ context.Context, _ ...r3.Query) (int64, error) {
	return int64(len(m.posts)), nil
}
func (m *memoryPostCRUD) Create(_ context.Context, p Post) (Post, error) { return p, nil }
func (m *memoryPostCRUD) Update(_ context.Context, p Post) (Post, error) { return p, nil }
func (m *memoryPostCRUD) Patch(_ context.Context, p Post, _ r3.Fields) (Post, error) {
	return p, nil
}
func (m *memoryPostCRUD) Delete(_ context.Context, _ int64) error { return nil }

// --- fixtures ----------------------------------------------------------

// newPostRepo builds a fresh repo per test. Fresh matters: the overlay writes
// through the pointers it is handed, so a shared fixture would leak between
// subtests the way it never would between two real reads.
func newPostRepo(
	t *testing.T, posts []Post, rels ...i18n.Translated,
) (*i18n.CRUD[Post, int64], *memoryTranslationCRUD) {
	t.Helper()
	store := &memoryTranslationCRUD{}
	repo := i18n.WithTranslations[Post, int64](
		&memoryPostCRUD{posts: posts}, store,
		i18n.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
		i18n.WithFields[Post, int64]("title"),
		i18n.WithPreloads[Post, int64](rels...),
	)
	return repo, store
}

func authorRelation() i18n.Translated {
	return i18n.TranslatedRelation("Author", func(a Author) int64 { return a.ID }, "name")
}

func tagsRelation() i18n.Translated {
	return i18n.TranslatedRelation("Tags", func(tg Tag) int64 { return tg.ID }, "label")
}

func TestPreloadOverlay(t *testing.T) {
	ru := func() context.Context { return r3.WithLocale(context.Background(), "ru") }

	t.Run("a pointer child is overlaid", func(t *testing.T) {
		repo, store := newPostRepo(t, []Post{
			{ID: 1, Title: "Unu", Author: &Author{ID: 7, Name: "Ann", Bio: "writes"}},
		}, authorRelation())
		seedTranslation(t, store, i18n.Translation{
			EntityType: "posts", EntityID: "1", Field: "title", Lang: "ru",
			Value: "Один", Source: i18n.SourceAI, SourceHash: i18n.Hash("Unu"),
		})
		seedTranslation(t, store, i18n.Translation{
			EntityType: "authors", EntityID: "7", Field: "name", Lang: "ru",
			Value: "Анна", Source: i18n.SourceAI, SourceHash: i18n.Hash("Ann"),
		})

		got, err := repo.Get(ru(), 1)
		be.NoError(t, err)
		be.AssertThat(t, got.Title, be.Eq("Один"))
		be.RequireThat(t, got.Author, be.Not(be.Nil()))
		be.AssertThat(t, got.Author.Name, be.Eq("Анна"))
		be.AssertThat(t, got.Author.Bio, be.Eq("writes"), "a field the relation does not name is untouched")
	})

	t.Run("a slice child is overlaid", func(t *testing.T) {
		repo, store := newPostRepo(t, []Post{
			{ID: 1, Title: "Unu", Tags: []Tag{{ID: 1, Label: "news"}, {ID: 2, Label: "sport"}}},
		}, tagsRelation())
		seedTranslation(t, store, i18n.Translation{
			EntityType: "tags", EntityID: "1", Field: "label", Lang: "ru",
			Value: "новости", Source: i18n.SourceAI, SourceHash: i18n.Hash("news"),
		})
		seedTranslation(t, store, i18n.Translation{
			EntityType: "tags", EntityID: "2", Field: "label", Lang: "ru",
			Value: "спорт", Source: i18n.SourceAI, SourceHash: i18n.Hash("sport"),
		})

		got, err := repo.Get(ru(), 1)
		be.NoError(t, err)
		be.RequireThat(t, got.Tags, be.HaveLength(2))
		be.AssertThat(t, got.Tags[0].Label, be.Eq("новости"))
		be.AssertThat(t, got.Tags[1].Label, be.Eq("спорт"))
		// Nothing was seeded for the post itself: a parent with no translations
		// of its own must still have its children overlaid.
		be.AssertThat(t, got.Title, be.Eq("Unu"))
	})

	t.Run("a nil pointer and an empty slice are left alone", func(t *testing.T) {
		repo, store := newPostRepo(t, []Post{
			{ID: 1, Title: "Unu"},
		}, authorRelation(), tagsRelation())

		store.lists = 0
		got, err := repo.Get(ru(), 1)
		be.NoError(t, err)
		be.AssertThat(t, got.Author, be.Nil())
		be.AssertThat(t, got.Tags, be.Empty())
		be.AssertThat(t, store.lists, be.Eq(1), "a relation with no children issues no query")
	})

	t.Run("a child with no translation keeps its source text", func(t *testing.T) {
		repo, store := newPostRepo(t, []Post{
			{ID: 1, Title: "Unu", Author: &Author{ID: 7, Name: "Ann"}},
		}, authorRelation())
		seedTranslation(t, store, i18n.Translation{
			EntityType: "posts", EntityID: "1", Field: "title", Lang: "ru",
			Value: "Один", Source: i18n.SourceAI, SourceHash: i18n.Hash("Unu"),
		})

		got, err := repo.Get(ru(), 1)
		be.NoError(t, err)
		be.AssertThat(t, got.Title, be.Eq("Один"))
		be.AssertThat(t, got.Author.Name, be.Eq("Ann"))
	})

	t.Run("an undeclared relation is untouched", func(t *testing.T) {
		// Author declared, Tags not.
		repo, store := newPostRepo(t, []Post{
			{ID: 1, Title: "Unu", Author: &Author{ID: 7, Name: "Ann"}, Tags: []Tag{{ID: 1, Label: "news"}}},
		}, authorRelation())
		seedTranslation(t, store, i18n.Translation{
			EntityType: "authors", EntityID: "7", Field: "name", Lang: "ru",
			Value: "Анна", Source: i18n.SourceAI, SourceHash: i18n.Hash("Ann"),
		})
		seedTranslation(t, store, i18n.Translation{
			EntityType: "tags", EntityID: "1", Field: "label", Lang: "ru",
			Value: "новости", Source: i18n.SourceAI, SourceHash: i18n.Hash("news"),
		})

		got, err := repo.Get(ru(), 1)
		be.NoError(t, err)
		be.AssertThat(t, got.Author.Name, be.Eq("Анна"))
		be.AssertThat(t, got.Tags[0].Label, be.Eq("news"), "an undeclared relation is never queried")
	})

	t.Run("without a locale nothing is queried", func(t *testing.T) {
		repo, store := newPostRepo(t, []Post{
			{ID: 1, Title: "Unu", Author: &Author{ID: 7, Name: "Ann"}},
		}, authorRelation())

		store.lists = 0
		got, err := repo.Get(context.Background(), 1)
		be.NoError(t, err)
		be.AssertThat(t, got.Author.Name, be.Eq("Ann"))
		be.AssertThat(t, store.lists, be.Eq(0))
	})
}

// The package doc promises the overlay is never N+1. A page of parents must cost
// one store query for the parents plus one per declared relation, whatever the
// page size, and a child hanging off several parents is asked for once.
func TestPreloadOverlayBatchesPerRelation(t *testing.T) {
	// Three posts, all by author 7, each with its own allocation of it the way a
	// driver's preload would hand them over.
	repo, store := newPostRepo(t, []Post{
		{ID: 1, Title: "Unu", Author: &Author{ID: 7, Name: "Ann"}, Tags: []Tag{{ID: 1, Label: "news"}}},
		{ID: 2, Title: "Doi", Author: &Author{ID: 7, Name: "Ann"}, Tags: []Tag{{ID: 2, Label: "sport"}}},
		{ID: 3, Title: "Trei", Author: &Author{ID: 7, Name: "Ann"}, Tags: []Tag{{ID: 1, Label: "news"}}},
	}, authorRelation(), tagsRelation())

	seedTranslation(t, store, i18n.Translation{
		EntityType: "authors", EntityID: "7", Field: "name", Lang: "ru",
		Value: "Анна", Source: i18n.SourceAI, SourceHash: i18n.Hash("Ann"),
	})
	seedTranslation(t, store, i18n.Translation{
		EntityType: "tags", EntityID: "1", Field: "label", Lang: "ru",
		Value: "новости", Source: i18n.SourceAI, SourceHash: i18n.Hash("news"),
	})

	store.lists = 0
	items, _, err := repo.List(r3.WithLocale(context.Background(), "ru"))
	be.NoError(t, err)
	be.RequireThat(t, items, be.HaveLength(3))

	be.AssertThat(t, store.lists, be.Eq(3), "one query for the parents, one per declared relation")

	// Every copy of the shared author is overlaid, not just the first.
	for i := range items {
		be.AssertThat(t, items[i].Author.Name, be.Eq("Анна"))
	}
	be.AssertThat(t, items[0].Tags[0].Label, be.Eq("новости"))
	be.AssertThat(t, items[1].Tags[0].Label, be.Eq("sport"), "no translation, so source text")
	be.AssertThat(t, items[2].Tags[0].Label, be.Eq("новости"))
}

func TestPreloadDeclarationPanicsOnMisconfiguration(t *testing.T) {
	wrap := func(rels ...i18n.Translated) func() {
		return func() {
			i18n.WithTranslations[Post, int64](
				&memoryPostCRUD{}, &memoryTranslationCRUD{},
				i18n.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
				i18n.WithFields[Post, int64]("title"),
				i18n.WithPreloads[Post, int64](rels...),
			)
		}
	}

	be.AssertThat(t, wrap(
		i18n.TranslatedRelation("Nope", func(a Author) int64 { return a.ID }, "name"),
	), be.Panic(), "unknown field: want panic, got none")

	be.AssertThat(t, wrap(
		i18n.TranslatedRelation("Tags", func(a Author) int64 { return a.ID }, "name"),
	), be.Panic(), "field holds Tag, relation declares Author: want panic, got none")

	be.AssertThat(t, wrap(
		i18n.TranslatedRelation("Author", func(a Author) int64 { return a.ID }, "bogus"),
	), be.Panic(), "unknown child field: want panic, got none")

	be.AssertThat(t, wrap(
		i18n.TranslatedRelation("Author", func(a Author) int64 { return a.ID }, "id"),
	), be.Panic(), "non-string child field: want panic, got none")

	be.AssertThat(t, wrap(
		i18n.TranslatedRelation("Author", func(a Author) int64 { return a.ID }),
	), be.Panic(), "no fields named: want panic, got none")
}
