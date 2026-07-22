package r3sqlite3_test

import (
	"context"
	"testing"
	"time"

	"github.com/amberpixels/r3"
	r3sqlite3 "github.com/amberpixels/r3/drivers/sqlite3"
	"github.com/stretchr/testify/require"
)

// widget exercises the schema capability model on the raw SQL engine:
//   - slug is immutable (set-once)
//   - population is a readonly feed column (worker-writable via bypass)
//   - created_at/updated_at are system-managed timestamps (engine-written)
//   - deleted_at is the soft-delete column
//   - secret is hidden from filter/sort/output
type widget struct {
	ID         int64      `db:"id,pk"`
	Title      string     `db:"title"`
	Slug       string     `db:"slug,immutable"`
	Population int        `db:"population,readonly"`
	Secret     string     `db:"secret,no-filter,no-sort,no-output"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	DeletedAt  *time.Time `db:"deleted_at,soft_delete"`
}

func setupWidgets(t *testing.T) *r3sqlite3.Sqlite3CRUD[widget, int64] {
	t.Helper()
	db, err := setupSQLiteDB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`CREATE TABLE widgets (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		title       TEXT NOT NULL,
		slug        TEXT NOT NULL,
		population  INTEGER NOT NULL DEFAULT 0,
		secret      TEXT NOT NULL DEFAULT '',
		created_at  DATETIME,
		updated_at  DATETIME,
		deleted_at  DATETIME
	)`)
	require.NoError(t, err)

	return r3sqlite3.NewSqlite3CRUD[widget, int64](db)
}

func TestSchema_ReadValidation_TypedErrors(t *testing.T) {
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
			// List and Count both validate before emitting SQL.
			_, _, err := repo.List(ctx, tc.query)
			require.ErrorIs(t, err, tc.wantErr)
			_, err = repo.Count(ctx, tc.query)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestSchema_Timestamps_Managed(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)
	// The engine stamps both managed timestamps with server time on Create.
	require.False(t, created.CreatedAt.IsZero(), "created_at must be set on Create")
	require.False(t, created.UpdatedAt.IsZero(), "updated_at must be set on Create")

	afterCreate, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // guarantee the update timestamp advances

	// A client round-trips the row with zeroed timestamps (e.g. lost in JSON).
	created.Title = "b"
	created.CreatedAt = time.Time{}
	created.UpdatedAt = time.Time{}
	_, err = repo.Update(ctx, created)
	require.NoError(t, err)

	afterUpdate, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)

	// created_at is immutable across updates; updated_at bumps on every write.
	require.Equal(t, afterCreate.CreatedAt, afterUpdate.CreatedAt, "created_at must not change on Update")
	require.True(t, afterUpdate.UpdatedAt.After(afterCreate.UpdatedAt), "updated_at must bump on Update")
}

func TestSchema_Patch_BumpsUpdatedAt(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)
	afterCreate, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	w.Title = "b"
	_, err = repo.Patch(ctx, w, r3.Fields{r3.NewFieldSpec("title")})
	require.NoError(t, err)

	afterPatch, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, "b", afterPatch.Title)
	require.Equal(t, afterCreate.CreatedAt, afterPatch.CreatedAt, "created_at must not change on Patch")
	require.True(t, afterPatch.UpdatedAt.After(afterCreate.UpdatedAt), "updated_at must bump on Patch")
}

// H11: Update returns the row as persisted. The engine writes only the mutable
// columns - correct - but used to hand back the caller's input, so a partial
// model reported zeroes for every column the write skipped.
func TestSchema_Update_ReturnsPersistedState(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	// A partial model, the way a request DTO arrives: no created_at, plus values
	// for columns the write will not touch.
	updated, err := repo.Update(ctx, widget{
		ID:         created.ID,
		Title:      "b",
		Slug:       "hacked",
		Population: 999,
	})
	require.NoError(t, err)

	require.Equal(t, "b", updated.Title, "the written column must reflect the new value")
	require.Equal(t, created.CreatedAt, updated.CreatedAt, "created_at must come back stored, not zeroed")
	require.Equal(t, "a", updated.Slug, "an immutable column must come back stored")
	require.Equal(t, 0, updated.Population, "a readonly column must come back stored")
	require.False(t, updated.UpdatedAt.IsZero(), "updated_at must come back stamped")
}

func TestSchema_Update_NotFound(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	_, err := repo.Update(ctx, widget{ID: 4242, Title: "ghost", Slug: "ghost"})
	require.ErrorIs(t, err, r3.ErrNotFound, "Update on a missing row must report the sentinel")
}

func TestSchema_Update_DoesNotResurrectSoftDeleted(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)
	require.NoError(t, repo.Delete(ctx, w.ID)) // soft-delete

	// Update the trashed row with a nil deleted_at (would resurrect under the
	// old "write all non-PK columns" behavior).
	w.Title = "b"
	w.DeletedAt = nil
	_, err = repo.Update(ctx, w)
	require.NoError(t, err)

	_, err = repo.Get(ctx, w.ID)
	require.ErrorIs(t, err, r3.ErrNotFound, "soft-deleted row must stay trashed after Update")
}

func TestSchema_Update_DoesNotWriteReadonly(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a", Population: 0})
	require.NoError(t, err)

	// A normal caller cannot move a readonly feed column via Update.
	w.Population = 999
	_, err = repo.Update(ctx, w)
	require.NoError(t, err)

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, 0, got.Population, "readonly column must not change via a normal Update")
}

func TestSchema_Patch_RejectsImmutableAndReadonly(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	w.Slug = "b"
	_, err = repo.Patch(ctx, w, r3.Fields{r3.NewFieldSpec("slug")})
	require.ErrorIs(t, err, r3.ErrInvalidPatchField, "immutable column is not patchable")

	w.Population = 5
	_, err = repo.Patch(ctx, w, r3.Fields{r3.NewFieldSpec("population")})
	require.ErrorIs(t, err, r3.ErrInvalidPatchField, "readonly column is not patchable")
}

func TestSchema_SystemWriter_WritesReadonlyColumn(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	// The audited worker door: a system writer may move the feed-synced column.
	w.Population = 1234
	_, err = r3.SystemWriter(repo).Update(ctx, w)
	require.NoError(t, err)

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, 1234, got.Population, "SystemWriter must write the readonly column")
}

func TestSchema_SystemWriter_StillRespectsStructuralFloor(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	// Even bypassed, the PK is identity — Patching it is rejected by the
	// structural floor, not the (bypassed) capability check.
	_, err = r3.SystemWriter(repo).Patch(ctx, w, r3.Fields{r3.NewFieldSpec("id")})
	require.ErrorIs(t, err, r3.ErrInvalidPatchField)
}

func TestSchema_WithoutWriteGuard_DirectContext(t *testing.T) {
	repo := setupWidgets(t)
	ctx := context.Background()

	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	// The context marker is the primitive SystemWriter builds on.
	w.Population = 77
	_, err = repo.Update(r3.WithoutWriteGuard(ctx), w)
	require.NoError(t, err)

	got, err := repo.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, 77, got.Population)
}
