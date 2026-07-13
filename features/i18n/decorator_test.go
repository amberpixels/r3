package i18n_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/i18n"
	"github.com/expectto/be"
)

// Article is the entity under test: two translatable string fields (one via
// db tag, one via snake_case derivation), one untouched field.
type Article struct {
	ID    int64  `db:"id,pk"`
	Title string `db:"title"`
	// BodyText has no db tag — resolved as "body_text" via snake_case.
	BodyText string
	Views    int64 `db:"views"`
}

// --- in-memory fakes ---------------------------------------------------

type memoryArticleCRUD struct {
	mu       sync.Mutex
	articles map[int64]Article
}

func newMemoryArticleCRUD(items ...Article) *memoryArticleCRUD {
	m := &memoryArticleCRUD{articles: map[int64]Article{}}
	for _, a := range items {
		m.articles[a.ID] = a
	}
	return m
}

func (m *memoryArticleCRUD) Get(_ context.Context, id int64, _ ...r3.Query) (Article, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.articles[id]
	if !ok {
		return Article{}, r3.ErrNotFound
	}
	return a, nil
}

func (m *memoryArticleCRUD) List(_ context.Context, _ ...r3.Query) ([]Article, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Article, 0, len(m.articles))
	// Deterministic order by ID for assertions.
	for id := range int64(101) {
		if a, ok := m.articles[id]; ok {
			out = append(out, a)
		}
	}
	return out, int64(len(out)), nil
}

func (m *memoryArticleCRUD) Count(_ context.Context, _ ...r3.Query) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.articles)), nil
}

func (m *memoryArticleCRUD) Create(_ context.Context, a Article) (Article, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.articles[a.ID] = a
	return a, nil
}

func (m *memoryArticleCRUD) Update(_ context.Context, a Article) (Article, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.articles[a.ID] = a
	return a, nil
}

func (m *memoryArticleCRUD) Patch(ctx context.Context, a Article, _ r3.Fields) (Article, error) {
	return m.Update(ctx, a)
}

func (m *memoryArticleCRUD) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.articles, id)
	return nil
}

// memoryTranslationCRUD interprets the filter shapes produced by the i18n
// query builders (a single And group of Eq/In conditions).
type memoryTranslationCRUD struct {
	mu    sync.Mutex
	rows  []i18n.Translation
	lists int // number of List calls, to assert batching
}

func (s *memoryTranslationCRUD) match(tr i18n.Translation, filters r3.Filters) bool {
	for _, f := range filters {
		if !s.matchSpec(tr, f) {
			return false
		}
	}
	return true
}

func (s *memoryTranslationCRUD) matchSpec(tr i18n.Translation, f *r3.FilterSpec) bool {
	if len(f.And) > 0 {
		for _, g := range f.And {
			if !s.matchSpec(tr, g) {
				return false
			}
		}
		return true
	}

	var actual any
	switch f.Field.String() {
	case "entity_type":
		actual = tr.EntityType
	case "entity_id":
		actual = tr.EntityID
	case "field":
		actual = tr.Field
	case "lang":
		actual = tr.Lang
	case "stale":
		actual = tr.Stale
	default:
		return false
	}

	switch f.Operator { //nolint:exhaustive // the i18n query builders only emit Eq and In
	case r3.OperatorEq:
		return fmt.Sprint(actual) == fmt.Sprint(f.Value)
	case r3.OperatorIn:
		if vals, ok := f.Value.([]string); ok {
			return slices.Contains(vals, fmt.Sprint(actual))
		}
		return false
	default:
		return false
	}
}

func (s *memoryTranslationCRUD) Get(_ context.Context, id string, _ ...r3.Query) (i18n.Translation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, tr := range s.rows {
		if tr.ID == id {
			return tr, nil
		}
	}
	return i18n.Translation{}, r3.ErrNotFound
}

func (s *memoryTranslationCRUD) List(_ context.Context, qargs ...r3.Query) ([]i18n.Translation, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lists++
	var out []i18n.Translation
	for _, tr := range s.rows {
		if len(qargs) == 0 || s.match(tr, qargs[0].Filters) {
			out = append(out, tr)
		}
	}
	return out, int64(len(out)), nil
}

func (s *memoryTranslationCRUD) Count(ctx context.Context, qargs ...r3.Query) (int64, error) {
	_, n, err := s.List(ctx, qargs...)
	return n, err
}

func (s *memoryTranslationCRUD) Create(_ context.Context, tr i18n.Translation) (i18n.Translation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, tr)
	return tr, nil
}

func (s *memoryTranslationCRUD) Update(_ context.Context, tr i18n.Translation) (i18n.Translation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rows {
		if s.rows[i].ID == tr.ID {
			s.rows[i] = tr
			return tr, nil
		}
	}
	return tr, r3.ErrNotFound
}

func (s *memoryTranslationCRUD) Patch(ctx context.Context, tr i18n.Translation, _ r3.Fields) (i18n.Translation, error) {
	return s.Update(ctx, tr)
}

func (s *memoryTranslationCRUD) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rows {
		if s.rows[i].ID == id {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			return nil
		}
	}
	return r3.ErrNotFound
}

// --- helpers -----------------------------------------------------------

func newRepo(
	inner *memoryArticleCRUD, store *memoryTranslationCRUD, extra ...i18n.Option[Article, int64],
) *i18n.CRUD[Article, int64] {
	opts := append([]i18n.Option[Article, int64]{
		i18n.WithIDFunc(func(a Article) int64 { return a.ID }),
		i18n.WithFields[Article, int64]("title", "body_text"),
	}, extra...)
	return i18n.WithTranslations(inner, store, opts...)
}

func seedTranslation(t *testing.T, store *memoryTranslationCRUD, tr i18n.Translation) i18n.Translation {
	t.Helper()
	row, err := i18n.Upsert(context.Background(), store, tr)
	be.NoError(t, err)
	return row
}

// --- tests ---------------------------------------------------------------

func TestGetOverlaysContextLocale(t *testing.T) {
	inner := newMemoryArticleCRUD(Article{ID: 1, Title: "Trotuar blocat", BodyText: "Mașini pe trotuar", Views: 7})
	store := &memoryTranslationCRUD{}
	repo := newRepo(inner, store)

	seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Заблокированный тротуар", Source: i18n.SourceAI, SourceHash: i18n.Hash("Trotuar blocat"),
	})

	// No locale: source text untouched.
	got, err := repo.Get(context.Background(), 1)
	be.NoError(t, err)
	be.AssertThat(t, got.Title, be.Eq("Trotuar blocat"))

	// ru locale: title overlaid, untranslated body falls back to source.
	got, err = repo.Get(r3.WithLocale(context.Background(), "ru"), 1)
	be.NoError(t, err)
	be.AssertThat(t, got.Title, be.Eq("Заблокированный тротуар"))
	be.AssertThat(t, got.BodyText, be.Eq("Mașini pe trotuar"))
	be.AssertThat(t, got.Views, be.Eq(int64(7)))

	// A locale with no rows at all: everything falls back.
	got, _ = repo.Get(r3.WithLocale(context.Background(), "en"), 1)
	be.AssertThat(t, got.Title, be.Eq("Trotuar blocat"))
}

func TestListOverlaysInSingleStoreQuery(t *testing.T) {
	inner := newMemoryArticleCRUD(
		Article{ID: 1, Title: "Unu"},
		Article{ID: 2, Title: "Doi"},
		Article{ID: 3, Title: "Trei"},
	)
	store := &memoryTranslationCRUD{}
	repo := newRepo(inner, store)

	seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Один", Source: i18n.SourceAI, SourceHash: i18n.Hash("Unu"),
	})
	seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "3", Field: "title", Lang: "ru",
		Value: "Три", Source: i18n.SourceHuman, SourceHash: i18n.Hash("Trei"),
	})

	store.lists = 0
	items, total, err := repo.List(r3.WithLocale(context.Background(), "ru"))
	be.NoError(t, err)
	be.RequireThat(t, total, be.Eq(int64(3)))
	be.RequireThat(t, items, be.HaveLength(3))
	be.AssertThat(t, items[0].Title, be.Eq("Один"))
	be.AssertThat(t, items[1].Title, be.Eq("Doi"))
	be.AssertThat(t, items[2].Title, be.Eq("Три"))
	be.AssertThat(t, store.lists, be.Eq(1))

	// Without a locale the store must not be queried at all.
	store.lists = 0
	_, _, err = repo.List(context.Background())
	be.NoError(t, err)
	be.AssertThat(t, store.lists, be.Eq(0))
}

func TestUpdateMarksChangedFieldsStale(t *testing.T) {
	inner := newMemoryArticleCRUD(Article{ID: 1, Title: "Vechi", BodyText: "Corp"})
	store := &memoryTranslationCRUD{}
	repo := newRepo(inner, store)

	titleTr := seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Старый", Source: i18n.SourceAI, SourceHash: i18n.Hash("Vechi"),
	})
	bodyTr := seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "body_text", Lang: "ru",
		Value: "Текст", Source: i18n.SourceAI, SourceHash: i18n.Hash("Corp"),
	})

	// Change only the title.
	_, err := repo.Update(context.Background(), Article{ID: 1, Title: "Nou", BodyText: "Corp"})
	be.NoError(t, err)

	gotTitle, _ := store.Get(context.Background(), titleTr.ID)
	be.AssertThat(t, gotTitle.Stale, be.True())
	gotBody, _ := store.Get(context.Background(), bodyTr.ID)
	be.AssertThat(t, gotBody.Stale, be.False())

	// Stale translations are still served by default...
	got, _ := repo.Get(r3.WithLocale(context.Background(), "ru"), 1)
	be.AssertThat(t, got.Title, be.Eq("Старый"))

	// ...and hidden with WithoutStale.
	strict := newRepo(inner, store, i18n.WithoutStale[Article, int64]())
	got, _ = strict.Get(r3.WithLocale(context.Background(), "ru"), 1)
	be.AssertThat(t, got.Title, be.Eq("Nou"))
}

func TestUpsertReplacesAndClearsStale(t *testing.T) {
	store := &memoryTranslationCRUD{}
	ctx := context.Background()

	first := seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Первый", Source: i18n.SourceAI, SourceHash: i18n.Hash("v1"),
	})

	// Simulate staleness, then re-translate via Upsert.
	first.Stale = true
	_, err := store.Update(ctx, first)
	be.NoError(t, err)
	second, err := i18n.Upsert(ctx, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Второй", Source: i18n.SourceAI, SourceHash: i18n.Hash("v2"),
	})
	be.NoError(t, err)

	be.AssertThat(t, second.ID, be.Eq(first.ID))
	be.AssertThat(t, second.Stale, be.False())
	be.AssertThat(t, second.Value, be.Eq("Второй"))
	n, _ := store.Count(ctx)
	be.AssertThat(t, n, be.Eq(int64(1)))
}

func TestDeleteCleanupIsOptIn(t *testing.T) {
	ctx := context.Background()

	// Default: translations survive entity deletion.
	inner := newMemoryArticleCRUD(Article{ID: 1, Title: "Unu"})
	store := &memoryTranslationCRUD{}
	repo := newRepo(inner, store)
	seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Один", Source: i18n.SourceAI, SourceHash: i18n.Hash("Unu"),
	})
	err := repo.Delete(ctx, 1)
	be.NoError(t, err)
	n, _ := store.Count(ctx)
	be.AssertThat(t, n, be.Eq(int64(1)))

	// Opt-in: translations go with the entity.
	inner = newMemoryArticleCRUD(Article{ID: 2, Title: "Doi"})
	store = &memoryTranslationCRUD{}
	repo = newRepo(inner, store, i18n.WithDeleteOnEntityDelete[Article, int64]())
	seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "2", Field: "title", Lang: "ru",
		Value: "Два", Source: i18n.SourceAI, SourceHash: i18n.Hash("Doi"),
	})
	err = repo.Delete(ctx, 2)
	be.NoError(t, err)
	n, _ = store.Count(ctx)
	be.AssertThat(t, n, be.Eq(int64(0)))
}

func TestWithTranslationsPanicsOnMisconfiguration(t *testing.T) {
	inner := newMemoryArticleCRUD()
	store := &memoryTranslationCRUD{}

	be.AssertThat(t, func() {
		i18n.WithTranslations(inner, store,
			i18n.WithFields[Article, int64]("title"))
	}, be.Panic(), "missing IDFunc: want panic, got none")
	be.AssertThat(t, func() {
		i18n.WithTranslations(inner, store,
			i18n.WithIDFunc(func(a Article) int64 { return a.ID }))
	}, be.Panic(), "missing fields: want panic, got none")
	be.AssertThat(t, func() {
		newRepo(inner, store, i18n.WithFields[Article, int64]("no_such_field"))
	}, be.Panic(), "unknown field: want panic, got none")
	be.AssertThat(t, func() {
		newRepo(inner, store, i18n.WithFields[Article, int64]("views"))
	}, be.Panic(), "non-string field: want panic, got none")
}

func TestWithoutOverlayStillMarksStale(t *testing.T) {
	inner := newMemoryArticleCRUD(Article{ID: 1, Title: "Vechi"})
	store := &memoryTranslationCRUD{}
	repo := newRepo(inner, store, i18n.WithoutOverlay[Article, int64]())

	tr := seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Старый", Source: i18n.SourceAI, SourceHash: i18n.Hash("Vechi"),
	})

	// Reads pass through even with a locale set — and never hit the store.
	store.lists = 0
	got, err := repo.Get(r3.WithLocale(context.Background(), "ru"), 1)
	be.NoError(t, err)
	be.AssertThat(t, got.Title, be.Eq("Vechi"))
	be.AssertThat(t, store.lists, be.Eq(0))

	// Writes still invalidate.
	_, err = repo.Update(context.Background(), Article{ID: 1, Title: "Nou"})
	be.NoError(t, err)
	gotTr, _ := store.Get(context.Background(), tr.ID)
	be.AssertThat(t, gotTr.Stale, be.True())
}

func TestEmptyTranslationValueIsNeverOverlaid(t *testing.T) {
	inner := newMemoryArticleCRUD(Article{ID: 1, Title: "Sursă"})
	store := &memoryTranslationCRUD{}
	repo := newRepo(inner, store)

	// A row with an empty value (e.g. a failed worker write) must not blank
	// out the entity.
	store.rows = append(store.rows, i18n.Translation{
		ID: "empty", EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "", Source: i18n.SourceAI,
	})

	got, _ := repo.Get(r3.WithLocale(context.Background(), "ru"), 1)
	be.AssertThat(t, got.Title, be.Eq("Sursă"))
}
