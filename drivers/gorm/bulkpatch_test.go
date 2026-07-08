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

// job models the boot-recovery sweep use case: "mark every row still `running`
// as `interrupted`" is a single UPDATE ... WHERE status = 'running'.
type job struct {
	ID        int64  `r3:"id,pk"`
	Status    string `r3:"status"`
	Note      string `r3:"note"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func setupJobs(t *testing.T) *r3gorm.GormCRUD[job, int64] {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&job{}))
	return r3gorm.NewGormCRUD[job, int64](db)
}

func seedJobs(t *testing.T, repo *r3gorm.GormCRUD[job, int64], statuses ...string) []job {
	t.Helper()
	ctx := context.Background()
	out := make([]job, 0, len(statuses))
	for _, s := range statuses {
		j, err := repo.Create(ctx, job{Status: s})
		require.NoError(t, err)
		out = append(out, j)
	}
	return out
}

func TestGormPatchWhere_UpdatesOnlyMatching(t *testing.T) {
	repo := setupJobs(t)
	ctx := context.Background()
	seedJobs(t, repo, "running", "running", "done")

	n, err := repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("status", "running")},
		job{Status: "interrupted"},
		r3.Fields{r3.NewFieldSpec("status")},
	)
	require.NoError(t, err)
	require.Equal(t, int64(2), n, "only the two running rows are affected")

	interrupted, err := repo.Count(ctx, r3.Query{Filters: r3.Filters{r3.Eq("status", "interrupted")}})
	require.NoError(t, err)
	require.Equal(t, int64(2), interrupted)

	done, err := repo.Count(ctx, r3.Query{Filters: r3.Filters{r3.Eq("status", "done")}})
	require.NoError(t, err)
	require.Equal(t, int64(1), done, "non-matching row untouched")
}

func TestGormPatchWhere_BumpsUpdatedAt(t *testing.T) {
	repo := setupJobs(t)
	ctx := context.Background()
	seeded := seedJobs(t, repo, "running")

	time.Sleep(10 * time.Millisecond)
	_, err := repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("status", "running")},
		job{Status: "interrupted"},
		r3.Fields{r3.NewFieldSpec("status")},
	)
	require.NoError(t, err)

	got, err := repo.Get(ctx, seeded[0].ID)
	require.NoError(t, err)
	require.Equal(t, "interrupted", got.Status)
	require.True(t, got.UpdatedAt.After(seeded[0].UpdatedAt), "updated_at must bump")
}

func TestGormPatchWhere_OnlyWritesSelectedFields(t *testing.T) {
	repo := setupJobs(t)
	ctx := context.Background()
	seeded := seedJobs(t, repo, "running")
	// Give the row a note so we can prove PatchWhere doesn't clobber it.
	_, err := repo.Patch(ctx, job{ID: seeded[0].ID, Note: "keep"}, r3.Fields{r3.NewFieldSpec("note")})
	require.NoError(t, err)

	_, err = repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("status", "running")},
		job{Status: "interrupted", Note: "should-not-apply"},
		r3.Fields{r3.NewFieldSpec("status")},
	)
	require.NoError(t, err)

	got, err := repo.Get(ctx, seeded[0].ID)
	require.NoError(t, err)
	require.Equal(t, "interrupted", got.Status)
	require.Equal(t, "keep", got.Note, "column outside the Fields list must be preserved")
}

func TestGormPatchWhere_RejectsNonMutableColumn(t *testing.T) {
	repo := setupJobs(t)
	ctx := context.Background()
	seedJobs(t, repo, "running")

	_, err := repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("status", "running")},
		job{},
		r3.Fields{r3.NewFieldSpec("id")}, // PK is not patchable
	)
	require.ErrorIs(t, err, r3.ErrInvalidPatchField)
}

func TestGormPatchWhere_NoMatchesIsZero(t *testing.T) {
	repo := setupJobs(t)
	ctx := context.Background()
	seedJobs(t, repo, "done")

	n, err := repo.PatchWhere(ctx,
		r3.Filters{r3.Eq("status", "running")},
		job{Status: "interrupted"},
		r3.Fields{r3.NewFieldSpec("status")},
	)
	require.NoError(t, err)
	require.Equal(t, int64(0), n)
}
