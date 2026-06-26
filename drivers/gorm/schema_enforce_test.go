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

// widget mirrors the raw-engine schema test on the GORM seam, proving the engine
// (not the driver) carries read validation and write-shaping. GORM maps fields
// to snake_case columns by default; the r3 tags drive the capabilities.
type widget struct {
	ID         int64  `r3:"id,pk"`
	Title      string `r3:"title"`
	Slug       string `r3:"slug,immutable"`
	Population int    `r3:"population,readonly"`
	Secret     string `r3:"secret,no-filter,no-sort,no-output"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `r3:"deleted_at,soft_delete"`
}

func setupWidgets(t *testing.T) *r3gorm.GormCRUD[widget, int64] {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&widget{}))
	return r3gorm.NewGormCRUD[widget, int64](db)
}

func TestGormSchema_ReadValidation_TypedErrors(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	cases := []struct {
		name    string
		query   r3.Query
		wantErr error
	}{
		{"non-filterable", r3.Query{Filters: r3.Filters{r3.Eq("secret", "x")}}, r3.ErrFieldNotFilterable},
		{
			"non-sortable",
			r3.Query{Sorts: r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("secret"))}},
			r3.ErrFieldNotSortable,
		},
		{"non-queryable", r3.Query{Fields: r3.Fields{r3.NewFieldSpec("secret")}}, r3.ErrFieldNotQueryable},
		{"unknown", r3.Query{Filters: r3.Filters{r3.Eq("bogus", 1)}}, r3.ErrUnknownField},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := repo.List(ctx, tc.query)
			require.ErrorIs(t, err, tc.wantErr)
			_, err = repo.Count(ctx, tc.query)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestGormSchema_Timestamps_Managed(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)
	require.False(t, created.CreatedAt.IsZero(), "created_at must be set on Create")
	require.False(t, created.UpdatedAt.IsZero(), "updated_at must be set on Create")

	afterCreate, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	created.Title = "b"
	created.CreatedAt = time.Time{}
	created.UpdatedAt = time.Time{}
	_, err = repo.Update(ctx, created)
	require.NoError(t, err)

	afterUpdate, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	require.WithinDuration(t, afterCreate.CreatedAt, afterUpdate.CreatedAt, time.Millisecond,
		"created_at must not change on Update")
	require.True(t, afterUpdate.UpdatedAt.After(afterCreate.UpdatedAt), "updated_at must bump on Update")
}

func TestGormSchema_Update_DoesNotWriteReadonly(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	w.Population = 999
	_, err = repo.Update(ctx, w)
	require.NoError(t, err)

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, 0, got.Population, "readonly column must not change via a normal Update")
}

func TestGormSchema_Patch_RejectsImmutableAndReadonly(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	_, err = repo.Patch(ctx, w, r3.Fields{r3.NewFieldSpec("slug")})
	require.ErrorIs(t, err, r3.ErrInvalidPatchField)

	_, err = repo.Patch(ctx, w, r3.Fields{r3.NewFieldSpec("population")})
	require.ErrorIs(t, err, r3.ErrInvalidPatchField)
}

func TestGormSchema_SystemWriter_WritesReadonlyColumn(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	w.Population = 1234
	_, err = r3.SystemWriter(repo).Update(ctx, w)
	require.NoError(t, err)

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, 1234, got.Population, "SystemWriter must write the readonly column")
}

func TestGormSchema_SystemWriter_StillRespectsStructuralFloor(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	_, err = r3.SystemWriter(repo).Patch(ctx, w, r3.Fields{r3.NewFieldSpec("id")})
	require.ErrorIs(t, err, r3.ErrInvalidPatchField, "PK is unwritable even under bypass")
}
