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

// H11: Update must return the persisted row, not the caller's input. GORM omits
// the immutable columns from the write - correct, the row keeps them - but then
// returned the input unchanged, so a partial model (a request DTO with no
// created_at) came back with a zero created_at while the DB held the real one.
// The history feature diffs that return value and recorded a phantom change.
func TestGormSchema_Update_ReturnsPersistedState(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	stored, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)

	// A partial model, the way a request DTO arrives: no created_at, plus values
	// for columns the write is going to omit.
	updated, err := repo.Update(ctx, widget{
		ID:         created.ID,
		Title:      "b",
		Slug:       "hacked",
		Population: 999,
	})
	require.NoError(t, err)

	require.Equal(t, "b", updated.Title, "the written column must reflect the new value")
	require.False(t, updated.CreatedAt.IsZero(), "created_at must come back from the DB, not from the zero input")
	require.WithinDuration(t, stored.CreatedAt, updated.CreatedAt, time.Millisecond,
		"created_at must be the stored value")
	require.Equal(t, "a", updated.Slug, "an immutable column the write omitted must reflect the stored value")
	require.Equal(t, 0, updated.Population, "a readonly column the write omitted must reflect the stored value")
	require.False(t, updated.UpdatedAt.IsZero(), "updated_at must come back stamped")
}

// The post-write refresh reads the row directly, not through the repo's default
// Get query: a default that selects a subset of fields would otherwise fold
// zeros over every column it leaves out.
func TestGormSchema_Update_IgnoresDefaultGetQuery(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	repo.SetDefaultGetQuery(r3.Query{Fields: r3.Fields{r3.NewFieldSpec("title")}})

	updated, err := repo.Update(ctx, widget{ID: created.ID, Title: "b", Slug: "a"})
	require.NoError(t, err)

	require.Equal(t, "b", updated.Title)
	require.False(t, updated.CreatedAt.IsZero(), "a narrow default Get query must not zero the refreshed columns")
	require.Equal(t, "a", updated.Slug, "a narrow default Get query must not zero the refreshed columns")
}

// gizmo pairs a belongs-to pointer with column fields: the post-write refresh
// must not drop it. history diffs pointer-to-struct fields (only slices and maps
// are skipped), so returning a nil association would swap one phantom diff for
// another.
type gizmo struct {
	ID        int64  `r3:"id,pk"`
	Name      string `r3:"name"`
	OwnerID   int64  `r3:"owner_id"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Owner *gizmoOwner `gorm:"foreignKey:OwnerID"`
}

type gizmoOwner struct {
	ID   int64  `r3:"id,pk"`
	Name string `r3:"name"`
}

func TestGormSchema_Update_PreservesAssociations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&gizmoOwner{}, &gizmo{}))

	owners := r3gorm.NewGormCRUD[gizmoOwner, int64](db)
	repo := r3gorm.NewGormCRUD[gizmo, int64](db)
	ctx := context.Background()

	owner, err := owners.Create(ctx, gizmoOwner{Name: "acme"})
	require.NoError(t, err)

	g, err := repo.Create(ctx, gizmo{Name: "a", OwnerID: owner.ID})
	require.NoError(t, err)

	g.Name = "b"
	g.Owner = &gizmoOwner{ID: owner.ID, Name: "acme"}
	updated, err := repo.Update(ctx, g)
	require.NoError(t, err)

	require.Equal(t, "b", updated.Name)
	require.NotNil(t, updated.Owner, "an association the caller supplied must survive the post-write refresh")
	require.Equal(t, "acme", updated.Owner.Name)
}

func TestGormSchema_SystemWriter_StillRespectsStructuralFloor(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	_, err = r3.SystemWriter(repo).Patch(ctx, w, r3.Fields{r3.NewFieldSpec("id")})
	require.ErrorIs(t, err, r3.ErrInvalidPatchField, "PK is unwritable even under bypass")
}
