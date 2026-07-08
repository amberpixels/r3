package r3gorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setting is a key-value row keyed by a caller-supplied string PK — the
// canonical Upsert use case (settings/config stores).
type setting struct {
	Key       string `r3:"key,pk" gorm:"primaryKey"`
	Value     string `r3:"value"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func setupSettings(t *testing.T) *r3gorm.GormCRUD[setting, string] {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&setting{}))
	return r3gorm.NewGormCRUD[setting, string](db)
}

func TestGormUpsert_InsertThenUpdate(t *testing.T) {
	repo := setupSettings(t)
	ctx := context.Background()

	// Insert-when-absent.
	got, err := repo.Upsert(ctx, setting{Key: "theme", Value: "dark"}, r3.OnConflict("key"))
	require.NoError(t, err)
	require.Equal(t, "dark", got.Value)

	// Update-when-present: same key collides and overwrites.
	got, err = repo.Upsert(ctx, setting{Key: "theme", Value: "light"}, r3.OnConflict("key"))
	require.NoError(t, err)
	require.Equal(t, "light", got.Value)

	// Still a single row.
	n, err := repo.Count(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
}

func TestGormUpsert_DefaultConflictIsPK(t *testing.T) {
	repo := setupSettings(t)
	ctx := context.Background()

	_, err := repo.Upsert(ctx, setting{Key: "k", Value: "v1"})
	require.NoError(t, err)
	// No OnConflict option → conflict target defaults to the PK (key).
	_, err = repo.Upsert(ctx, setting{Key: "k", Value: "v2"})
	require.NoError(t, err)

	got, err := repo.Get(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, "v2", got.Value)
}

func TestGormUpsert_ManagedTimestamps(t *testing.T) {
	repo := setupSettings(t)
	ctx := context.Background()

	inserted, err := repo.Upsert(ctx, setting{Key: "k", Value: "v1"})
	require.NoError(t, err)
	require.False(t, inserted.CreatedAt.IsZero(), "created_at set on insert")
	require.False(t, inserted.UpdatedAt.IsZero(), "updated_at set on insert")

	time.Sleep(10 * time.Millisecond)

	updated, err := repo.Upsert(ctx, setting{Key: "k", Value: "v2"})
	require.NoError(t, err)
	require.WithinDuration(t, inserted.CreatedAt, updated.CreatedAt, time.Millisecond,
		"created_at must not change on the conflict-update branch")
	require.True(t, updated.UpdatedAt.After(inserted.UpdatedAt),
		"updated_at must bump on the conflict-update branch")
}

func TestGormUpsert_DefaultReplacesOnlyMutableColumns(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	orig, err := repo.Create(ctx, widget{Title: "a", Slug: "slug-a", Secret: "s1"})
	require.NoError(t, err)
	// Population is readonly; set it out-of-band so we can prove it survives.
	_, err = r3.SystemWriter(repo).Update(ctx, widget{
		ID: orig.ID, Title: "a", Slug: "slug-a", Secret: "s1", Population: 42,
	})
	require.NoError(t, err)

	// Upsert conflicting on the PK, replacing all mutable columns.
	_, err = repo.Upsert(ctx, widget{ID: orig.ID, Title: "b", Secret: "s2"}, r3.OnConflict("id"))
	require.NoError(t, err)

	got, err := repo.Get(ctx, orig.ID)
	require.NoError(t, err)
	require.Equal(t, "b", got.Title, "mutable column replaced")
	require.Equal(t, "s2", got.Secret, "mutable column replaced")
	require.Equal(t, "slug-a", got.Slug, "immutable column preserved")
	require.Equal(t, 42, got.Population, "readonly column preserved")
}

func TestGormUpsert_UpdateOnConflictSubset(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	orig, err := repo.Create(ctx, widget{Title: "a", Slug: "slug-a", Secret: "s1"})
	require.NoError(t, err)

	// Only "title" is overwritten on conflict; "secret" must stay untouched.
	_, err = repo.Upsert(ctx,
		widget{ID: orig.ID, Title: "b", Secret: "should-not-apply"},
		r3.OnConflict("id"),
		r3.UpdateOnConflict(r3.NewFieldSpec("title")),
	)
	require.NoError(t, err)

	got, err := repo.Get(ctx, orig.ID)
	require.NoError(t, err)
	require.Equal(t, "b", got.Title)
	require.Equal(t, "s1", got.Secret, "column outside UpdateOnConflict must be preserved")
}

func TestGormUpsert_RejectsNonMutableUpdateColumn(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	orig, err := repo.Create(ctx, widget{Title: "a", Slug: "slug-a"})
	require.NoError(t, err)

	_, err = repo.Upsert(ctx, widget{ID: orig.ID, Slug: "x"},
		r3.OnConflict("id"), r3.UpdateOnConflict(r3.NewFieldSpec("slug")))
	require.ErrorIs(t, err, r3.ErrInvalidPatchField, "immutable column rejected")

	_, err = repo.Upsert(ctx, widget{ID: orig.ID, Population: 9},
		r3.OnConflict("id"), r3.UpdateOnConflict(r3.NewFieldSpec("population")))
	require.ErrorIs(t, err, r3.ErrInvalidPatchField, "readonly column rejected")
}
