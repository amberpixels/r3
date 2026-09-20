package r3gorm_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
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

// usage is the counter shape: a key, a running total, and a column overwritten
// rather than accumulated on the same write.
type usage struct {
	Model    string `r3:"model,pk"  gorm:"primaryKey"`
	Tokens   int64  `r3:"tokens"`
	LastSeen string `r3:"last_seen"`
}

func setupUsage(t *testing.T) *r3gorm.GormCRUD[usage, string] {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&usage{}))
	return r3gorm.NewGormCRUD[usage, string](db)
}

func TestGormUpsert_IncrementOnConflict(t *testing.T) {
	ctx := context.Background()
	tokens := r3.NewFieldSpec("tokens")

	t.Run("first write stores the incoming value, later writes add to it", func(t *testing.T) {
		repo := setupUsage(t)

		got, err := repo.Upsert(ctx, usage{Model: "opus", Tokens: 100},
			r3.OnConflict("model"), r3.IncrementOnConflict(tokens))
		require.NoError(t, err)
		require.Equal(t, int64(100), got.Tokens)

		got, err = repo.Upsert(ctx, usage{Model: "opus", Tokens: 50},
			r3.OnConflict("model"), r3.IncrementOnConflict(tokens))
		require.NoError(t, err)
		require.Equal(t, int64(150), got.Tokens, "conflict branch adds rather than overwrites")
	})

	t.Run("other columns still overwrite on the same write", func(t *testing.T) {
		repo := setupUsage(t)

		_, err := repo.Upsert(ctx, usage{Model: "opus", Tokens: 1, LastSeen: "monday"},
			r3.OnConflict("model"), r3.IncrementOnConflict(tokens))
		require.NoError(t, err)
		got, err := repo.Upsert(ctx, usage{Model: "opus", Tokens: 1, LastSeen: "tuesday"},
			r3.OnConflict("model"), r3.IncrementOnConflict(tokens))
		require.NoError(t, err)
		require.Equal(t, int64(2), got.Tokens)
		require.Equal(t, "tuesday", got.LastSeen)
	})

	t.Run("a column cannot be both overwritten and incremented", func(t *testing.T) {
		_, err := setupUsage(t).Upsert(ctx, usage{Model: "opus", Tokens: 1},
			r3.OnConflict("model"),
			r3.UpdateOnConflict(tokens),
			r3.IncrementOnConflict(tokens),
		)
		require.ErrorIs(t, err, r3.ErrUpsertIncrementConflict)
	})
}

// quota names its own table, so GORM writes to "billing_quota" while the
// reflected meta name would be "quotas". The accumulating assignment qualifies
// the stored value with a table name, so it has to be the one GORM actually uses.
type quota struct {
	Account string `r3:"account,pk" gorm:"primaryKey"`
	Used    int64  `r3:"used"`
}

func (quota) TableName() string { return "billing_quota" }

func TestGormUpsert_IncrementOnCustomTableName(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&quota{}))
	repo := r3gorm.NewGormCRUD[quota, string](db)
	ctx := context.Background()

	_, err = repo.Upsert(ctx, quota{Account: "acme", Used: 5},
		r3.OnConflict("account"), r3.IncrementOnConflict(r3.NewFieldSpec("used")))
	require.NoError(t, err)

	got, err := repo.Upsert(ctx, quota{Account: "acme", Used: 7},
		r3.OnConflict("account"), r3.IncrementOnConflict(r3.NewFieldSpec("used")))
	require.NoError(t, err)
	require.Equal(t, int64(12), got.Used)
}
