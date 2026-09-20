package r3gorm_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// minted is the shape the Create return-value bug hid behind: a readonly column
// Create omits so the DB default fills it. The caller's value is discarded by
// design, so the stored one is the only correct thing to return.
type minted struct {
	ID     int64  `r3:"id,pk"           gorm:"primaryKey"`
	Name   string `r3:"name"`
	Serial string `r3:"serial,readonly" gorm:"default:'MINTED'"`
}

// keyed has a caller-assigned string PK, the case the zero-PK guard turns on.
type keyed struct {
	Code  string `r3:"code,pk"       gorm:"primaryKey"`
	Label string `r3:"label"`
	Tier  string `r3:"tier,readonly" gorm:"default:'standard'"`
}

func setupDB(t *testing.T, models ...any) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(models...))
	return db
}

func TestGormCreate_ReturnsPersistedRow(t *testing.T) {
	ctx := context.Background()

	t.Run("a column Create omits comes back as stored, not as passed", func(t *testing.T) {
		repo := r3gorm.NewGormCRUD[minted, int64](setupDB(t, &minted{}))

		created, err := repo.Create(ctx, minted{Name: "first", Serial: "ignored"})
		require.NoError(t, err)
		assert.Equal(t, "MINTED", created.Serial, "Create returned the input instead of the persisted row")

		// And the returned value agrees with a fresh read of the same row.
		got, err := repo.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, got.Serial, created.Serial)
		assert.Equal(t, got.Name, created.Name)
	})

	t.Run("the generated PK survives the refresh", func(t *testing.T) {
		repo := r3gorm.NewGormCRUD[minted, int64](setupDB(t, &minted{}))

		first, err := repo.Create(ctx, minted{Name: "a"})
		require.NoError(t, err)
		second, err := repo.Create(ctx, minted{Name: "b"})
		require.NoError(t, err)
		assert.NotZero(t, first.ID)
		assert.Greater(t, second.ID, first.ID, "the read-back must not clobber the generated PK")
	})

	t.Run("a caller-assigned string PK refreshes too", func(t *testing.T) {
		repo := r3gorm.NewGormCRUD[keyed, string](setupDB(t, &keyed{}))

		created, err := repo.Create(ctx, keyed{Code: "ABC", Label: "hello", Tier: "ignored"})
		require.NoError(t, err)
		assert.Equal(t, "ABC", created.Code)
		assert.Equal(t, "standard", created.Tier)
	})

	t.Run("an unset string PK skips the refresh instead of erroring", func(t *testing.T) {
		repo := r3gorm.NewGormCRUD[keyed, string](setupDB(t, &keyed{}))

		// Nothing generates the key, so there is no identity to re-read by. The
		// insert still succeeded, so Create must not report ErrNotFound.
		created, err := repo.Create(ctx, keyed{Label: "anonymous"})
		require.NoError(t, err)
		assert.Equal(t, "anonymous", created.Label)
	})
}
