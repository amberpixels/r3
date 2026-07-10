package i18n_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/i18n"
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
	if err != nil {
		t.Fatalf("seed Upsert: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Trotuar blocat" {
		t.Errorf("no-locale Title = %q, want source text", got.Title)
	}

	// ru locale: title overlaid, untranslated body falls back to source.
	got, err = repo.Get(r3.WithLocale(context.Background(), "ru"), 1)
	if err != nil {
		t.Fatalf("Get(ru): %v", err)
	}
	if got.Title != "Заблокированный тротуар" {
		t.Errorf("ru Title = %q, want translation", got.Title)
	}
	if got.BodyText != "Mașini pe trotuar" {
		t.Errorf("ru BodyText = %q, want source fallback", got.BodyText)
	}
	if got.Views != 7 {
		t.Errorf("Views = %d, want untouched 7", got.Views)
	}

	// A locale with no rows at all: everything falls back.
	got, _ = repo.Get(r3.WithLocale(context.Background(), "en"), 1)
	if got.Title != "Trotuar blocat" {
		t.Errorf("en Title = %q, want source fallback", got.Title)
	}
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
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("List = %d items (total %d), want 3", len(items), total)
	}
	if items[0].Title != "Один" || items[1].Title != "Doi" || items[2].Title != "Три" {
		t.Errorf("titles = %q, %q, %q - want overlay on 1 and 3 only",
			items[0].Title, items[1].Title, items[2].Title)
	}
	if store.lists != 1 {
		t.Errorf("store List calls = %d, want exactly 1 (batched overlay)", store.lists)
	}

	// Without a locale the store must not be queried at all.
	store.lists = 0
	if _, _, err := repo.List(context.Background()); err != nil {
		t.Fatalf("List (no locale): %v", err)
	}
	if store.lists != 0 {
		t.Errorf("store List calls without locale = %d, want 0", store.lists)
	}
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
	if _, err := repo.Update(context.Background(), Article{ID: 1, Title: "Nou", BodyText: "Corp"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	gotTitle, _ := store.Get(context.Background(), titleTr.ID)
	if !gotTitle.Stale {
		t.Error("title translation not marked stale after source change")
	}
	gotBody, _ := store.Get(context.Background(), bodyTr.ID)
	if gotBody.Stale {
		t.Error("body translation marked stale though its source is unchanged")
	}

	// Stale translations are still served by default...
	got, _ := repo.Get(r3.WithLocale(context.Background(), "ru"), 1)
	if got.Title != "Старый" {
		t.Errorf("default stale Title = %q, want stale translation served", got.Title)
	}

	// ...and hidden with WithoutStale.
	strict := newRepo(inner, store, i18n.WithoutStale[Article, int64]())
	got, _ = strict.Get(r3.WithLocale(context.Background(), "ru"), 1)
	if got.Title != "Nou" {
		t.Errorf("WithoutStale Title = %q, want source fallback", got.Title)
	}
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
	if _, err := store.Update(ctx, first); err != nil {
		t.Fatalf("mark stale: %v", err)
	}
	second, err := i18n.Upsert(ctx, store, i18n.Translation{
		EntityType: "articles", EntityID: "1", Field: "title", Lang: "ru",
		Value: "Второй", Source: i18n.SourceAI, SourceHash: i18n.Hash("v2"),
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if second.ID != first.ID {
		t.Errorf("Upsert created a new row (id %s), want update of %s", second.ID, first.ID)
	}
	if second.Stale {
		t.Error("Upsert left the row stale, want cleared")
	}
	if second.Value != "Второй" {
		t.Errorf("Value = %q, want replaced", second.Value)
	}
	if n, _ := store.Count(ctx); n != 1 {
		t.Errorf("row count = %d, want 1 (no duplicates)", n)
	}
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
	if err := repo.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if n, _ := store.Count(ctx); n != 1 {
		t.Errorf("default Delete removed translations, want kept (%d rows)", n)
	}

	// Opt-in: translations go with the entity.
	inner = newMemoryArticleCRUD(Article{ID: 2, Title: "Doi"})
	store = &memoryTranslationCRUD{}
	repo = newRepo(inner, store, i18n.WithDeleteOnEntityDelete[Article, int64]())
	seedTranslation(t, store, i18n.Translation{
		EntityType: "articles", EntityID: "2", Field: "title", Lang: "ru",
		Value: "Два", Source: i18n.SourceAI, SourceHash: i18n.Hash("Doi"),
	})
	if err := repo.Delete(ctx, 2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if n, _ := store.Count(ctx); n != 0 {
		t.Errorf("opt-in Delete kept %d translations, want 0", n)
	}
}

func TestWithTranslationsPanicsOnMisconfiguration(t *testing.T) {
	inner := newMemoryArticleCRUD()
	store := &memoryTranslationCRUD{}

	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: want panic, got none", name)
			}
		}()
		fn()
	}

	assertPanics("missing IDFunc", func() {
		i18n.WithTranslations(inner, store,
			i18n.WithFields[Article, int64]("title"))
	})
	assertPanics("missing fields", func() {
		i18n.WithTranslations(inner, store,
			i18n.WithIDFunc(func(a Article) int64 { return a.ID }))
	})
	assertPanics("unknown field", func() {
		newRepo(inner, store, i18n.WithFields[Article, int64]("no_such_field"))
	})
	assertPanics("non-string field", func() {
		newRepo(inner, store, i18n.WithFields[Article, int64]("views"))
	})
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
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Vechi" {
		t.Errorf("Title = %q, want untranslated source (overlay disabled)", got.Title)
	}
	if store.lists != 0 {
		t.Errorf("store List calls = %d, want 0 (overlay disabled)", store.lists)
	}

	// Writes still invalidate.
	if _, err := repo.Update(context.Background(), Article{ID: 1, Title: "Nou"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	gotTr, _ := store.Get(context.Background(), tr.ID)
	if !gotTr.Stale {
		t.Error("translation not marked stale by WithoutOverlay repo")
	}
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
	if got.Title != "Sursă" {
		t.Errorf("Title = %q, want source text (empty translation ignored)", got.Title)
	}
}
