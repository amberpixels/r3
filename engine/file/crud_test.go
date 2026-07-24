package enginefile_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/amberpixels/k1/maybe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	enginefile "github.com/amberpixels/r3/engine/file"
)

// City is a test entity.
type City struct {
	ID      int    `json:"id"      yaml:"id"      r3:"id,pk"`
	Name    string `json:"name"    yaml:"name"`
	Country string `json:"country" yaml:"country"`
}

// CityWithSoftDelete is a test entity with soft-delete support.
type CityWithSoftDelete struct {
	ID        int        `json:"id"         r3:"id,pk"`
	Name      string     `json:"name"`
	Country   string     `json:"country"`
	DeletedAt *time.Time `json:"deleted_at" r3:"deleted_at,soft_delete"`
}

// Pet is a test entity with string ID.
type Pet struct {
	ID   string `json:"id"   r3:"id,pk"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Age  int    `json:"age"`
}

// Scored exercises NULL-vs-zero handling: Score is a non-pointer int (a 0 is a
// real value, not NULL) and Optional is a pointer (nil is NULL).
type Scored struct {
	ID       int     `json:"id"       r3:"id,pk"`
	Score    int     `json:"score"`
	Optional *string `json:"optional"`
}

// TestExists_ZeroIsNotNull verifies M7: `exists` (IS NOT NULL) is true for a
// present non-pointer zero value and false only for a nil pointer.
func TestExists_ZeroIsNotNull(t *testing.T) {
	repo := newJSONRepo[Scored, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	set := "x"
	_, _ = repo.Create(ctx, Scored{Score: 0, Optional: nil}) // zero score, null optional
	_, _ = repo.Create(ctx, Scored{Score: 5, Optional: &set})

	// exists on a non-pointer int: a 0 still exists, so both records match.
	scoreExists, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.Exists("score", nil)},
	})
	require.NoError(t, err)
	assert.Len(t, scoreExists, 2, "a non-pointer zero value must count as existing")

	// exists on a pointer field: only the record with a non-nil pointer matches.
	optionalExists, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.Exists("optional", nil)},
	})
	require.NoError(t, err)
	assert.Len(t, optionalExists, 1, "a nil pointer is NULL and must not exist")
	assert.Equal(t, 5, optionalExists[0].Score)
}

// TestSort_ZeroNotTreatedAsNull verifies M7: a non-pointer zero value sorts as a
// real value, not pushed to the NULLS position.
func TestSort_ZeroNotTreatedAsNull(t *testing.T) {
	repo := newJSONRepo[Scored, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	for _, s := range []int{5, 0, 3} {
		_, err := repo.Create(ctx, Scored{Score: s})
		require.NoError(t, err)
	}

	// ASC with NULLS LAST: 0 is a real value and must sort FIRST (0,3,5). The old
	// behavior treated 0 as NULL and pushed it to the end (3,5,0).
	got, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Sorts: r3.Sorts{
			r3.NewSortSpec(r3.NewFieldSpec("score"), r3.SortDirectionAsc, r3.NullsPositionLast),
		},
	})
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, []int{0, 3, 5}, []int{got[0].Score, got[1].Score, got[2].Score})
}

func tempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func newJSONRepo[T any, ID comparable](t *testing.T, idGen enginefile.IDGenerator[ID]) *enginefile.BaseCRUD[T, ID] {
	t.Helper()
	dir := tempDir(t)
	repo, err := enginefile.New[T, ID](idGen,
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)
	return repo
}

func newYAMLRepo[T any, ID comparable](t *testing.T, idGen enginefile.IDGenerator[ID]) *enginefile.BaseCRUD[T, ID] {
	t.Helper()
	dir := tempDir(t)
	repo, err := enginefile.New[T, ID](idGen,
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.YAMLCodec()),
	)
	require.NoError(t, err)
	return repo
}

// --------------------------------------------------------------------------
// Create tests
// --------------------------------------------------------------------------

func TestCreate_JSON(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	city, err := repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)
	assert.Equal(t, 1, city.ID)
	assert.Equal(t, "Berlin", city.Name)

	city2, err := repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	require.NoError(t, err)
	assert.Equal(t, 2, city2.ID)
}

func TestCreate_YAML(t *testing.T) {
	repo := newYAMLRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	city, err := repo.Create(ctx, City{Name: "Tokyo", Country: "Japan"})
	require.NoError(t, err)
	assert.Equal(t, 1, city.ID)
}

func TestCreate_UUIDString(t *testing.T) {
	repo := newJSONRepo[Pet, string](t, enginefile.UUIDStringIDGen())
	ctx := context.Background()

	pet, err := repo.Create(ctx, Pet{Name: "Rex", Kind: "dog", Age: 3})
	require.NoError(t, err)
	assert.NotEmpty(t, pet.ID)
	assert.Len(t, pet.ID, 36) // UUID format: 8-4-4-4-12 = 36 chars
}

// --------------------------------------------------------------------------
// Get tests
// --------------------------------------------------------------------------

func TestGet(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	created, err := repo.Create(ctx, City{Name: "Paris", Country: "France"})
	require.NoError(t, err)

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Paris", got.Name)
	assert.Equal(t, "France", got.Country)
}

func TestGet_NotFound(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, err := repo.Get(ctx, 999)
	require.Error(t, err)
	assert.True(t, enginefile.IsNotFound(err))
	// The file engine normalizes its not-found error to the package-wide sentinel.
	assert.ErrorIs(t, err, r3.ErrNotFound)
}

// --------------------------------------------------------------------------
// List tests
// --------------------------------------------------------------------------

func TestList_All(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, err := repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, City{Name: "Paris", Country: "France"})
	require.NoError(t, err)

	cities, total, err := repo.List(ctx, r3.Query{Pagination: r3.NoPagination()})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, cities, 3)
}

func TestList_WithFilter(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Paris", Country: "France"})

	cities, total, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.F(r3.NewFieldSpec("country"), "Germany")},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, cities, 2)
	for _, c := range cities {
		assert.Equal(t, "Germany", c.Country)
	}
}

func TestList_WithSort(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Aachen", Country: "Germany"})

	cities, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Sorts:      r3.Sorts{r3.NewSortAscSpec(r3.NewFieldSpec("name"))},
	})
	require.NoError(t, err)
	require.Len(t, cities, 3)
	assert.Equal(t, "Aachen", cities[0].Name)
	assert.Equal(t, "Berlin", cities[1].Name)
	assert.Equal(t, "Munich", cities[2].Name)
}

func TestList_WithSortDesc(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Aachen", Country: "Germany"})

	cities, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Sorts:      r3.Sorts{r3.NewSortDescSpec(r3.NewFieldSpec("name"))},
	})
	require.NoError(t, err)
	require.Len(t, cities, 3)
	assert.Equal(t, "Munich", cities[0].Name)
	assert.Equal(t, "Berlin", cities[1].Name)
	assert.Equal(t, "Aachen", cities[2].Name)
}

func TestList_WithPagination(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	for i := range 10 {
		_, _ = repo.Create(ctx, City{Name: "City" + string(rune('A'+i)), Country: "Test"})
	}

	// Page 1, size 3
	cities, total, err := repo.List(ctx, r3.Query{
		Pagination: r3.NewPaginationSpec(1, 3),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10), total)
	assert.Len(t, cities, 3)

	// Page 2, size 3
	cities2, total2, err := repo.List(ctx, r3.Query{
		Pagination: r3.NewPaginationSpec(2, 3),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10), total2)
	assert.Len(t, cities2, 3)

	// Page 4, size 3 (last page, only 1 item)
	cities4, total4, err := repo.List(ctx, r3.Query{
		Pagination: r3.NewPaginationSpec(4, 3),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10), total4)
	assert.Len(t, cities4, 1)
}

func TestList_EmptyCollection(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	cities, total, err := repo.List(ctx, r3.Query{Pagination: r3.NoPagination()})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, cities)
}

func TestList_FilterLike(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Bremen", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Paris", Country: "France"})

	cities, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.FLike(r3.NewFieldSpec("name"), "B%")},
	})
	require.NoError(t, err)
	assert.Len(t, cities, 2)
}

func TestList_FilterIn(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Paris", Country: "France"})
	_, _ = repo.Create(ctx, City{Name: "Tokyo", Country: "Japan"})

	cities, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.Fop(r3.NewFieldSpec("country"), r3.OperatorIn, []string{"Germany", "Japan"})},
	})
	require.NoError(t, err)
	assert.Len(t, cities, 2)
}

func TestList_FilterAndOr(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, _ = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	_, _ = repo.Create(ctx, City{Name: "Paris", Country: "France"})

	// OR: country=France OR name=Berlin
	cities, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters: r3.Filters{
			r3.Or(
				r3.F(r3.NewFieldSpec("country"), "France"),
				r3.F(r3.NewFieldSpec("name"), "Berlin"),
			),
		},
	})
	require.NoError(t, err)
	assert.Len(t, cities, 2)
}

func TestList_FilterGt(t *testing.T) {
	repo := newJSONRepo[Pet, string](t, enginefile.UUIDStringIDGen())
	ctx := context.Background()

	_, _ = repo.Create(ctx, Pet{Name: "Rex", Kind: "dog", Age: 3})
	_, _ = repo.Create(ctx, Pet{Name: "Bella", Kind: "cat", Age: 5})
	_, _ = repo.Create(ctx, Pet{Name: "Max", Kind: "dog", Age: 7})

	pets, _, err := repo.List(ctx, r3.Query{
		Pagination: r3.NoPagination(),
		Filters:    r3.Filters{r3.Fop(r3.NewFieldSpec("age"), r3.OperatorGt, 4)},
	})
	require.NoError(t, err)
	assert.Len(t, pets, 2)
}

// --------------------------------------------------------------------------
// Update tests
// --------------------------------------------------------------------------

func TestUpdate(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	created, err := repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)

	created.Name = "New Berlin"
	updated, err := repo.Update(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "New Berlin", updated.Name)

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Berlin", got.Name)
}

func TestUpdate_NotFound(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	_, err := repo.Update(ctx, City{ID: 999, Name: "Ghost"})
	require.Error(t, err)
	assert.True(t, enginefile.IsNotFound(err))
}

// --------------------------------------------------------------------------
// Patch tests
// --------------------------------------------------------------------------

func TestPatch(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	created, err := repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)

	patched, err := repo.Patch(ctx, City{ID: created.ID, Name: "New Berlin"}, r3.Fields{r3.NewFieldSpec("name")})
	require.NoError(t, err)
	assert.Equal(t, "New Berlin", patched.Name)
	assert.Equal(t, "Germany", patched.Country) // unchanged

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "New Berlin", got.Name)
	assert.Equal(t, "Germany", got.Country)
}

// --------------------------------------------------------------------------
// Delete tests
// --------------------------------------------------------------------------

func TestDelete_Hard(t *testing.T) {
	repo := newJSONRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	created, err := repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)

	err = repo.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = repo.Get(ctx, created.ID)
	require.Error(t, err)
	assert.True(t, enginefile.IsNotFound(err))
}

func TestDelete_Soft(t *testing.T) {
	dir := tempDir(t)
	repo, err := enginefile.New[CityWithSoftDelete, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)
	ctx := context.Background()

	created, err := repo.Create(ctx, CityWithSoftDelete{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)

	err = repo.Delete(ctx, created.ID)
	require.NoError(t, err)

	// Should not be found without IncludeTrashed
	_, err = repo.Get(ctx, created.ID)
	require.Error(t, err)
	assert.True(t, enginefile.IsNotFound(err))

	// Should be found with IncludeTrashed
	got, err := repo.Get(ctx, created.ID, r3.Query{IncludeTrashed: maybe.Some(true)})
	require.NoError(t, err)
	assert.Equal(t, "Berlin", got.Name)
	assert.NotNil(t, got.DeletedAt)
}

func TestRestore(t *testing.T) {
	dir := tempDir(t)
	repo, err := enginefile.New[CityWithSoftDelete, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)
	ctx := context.Background()

	created, err := repo.Create(ctx, CityWithSoftDelete{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)

	err = repo.Delete(ctx, created.ID)
	require.NoError(t, err)

	err = repo.Restore(ctx, created.ID)
	require.NoError(t, err)

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Berlin", got.Name)
	assert.Nil(t, got.DeletedAt)
}

// --------------------------------------------------------------------------
// Directory mode tests
// --------------------------------------------------------------------------

func TestDirectoryMode(t *testing.T) {
	dir := tempDir(t)
	repo, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
		enginefile.WithDirectoryMode(),
	)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, City{Name: "Munich", Country: "Germany"})
	require.NoError(t, err)

	// Verify files exist
	_, err = os.Stat(filepath.Join(dir, "cities", "1.json"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, "cities", "2.json"))
	require.NoError(t, err)

	// List should work
	cities, total, err := repo.List(ctx, r3.Query{Pagination: r3.NoPagination()})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, cities, 2)
}

// TestDirectoryMode_PathTraversal verifies that a crafted string id cannot
// escape the storage directory when used as a filename (C5).
func TestDirectoryMode_PathTraversal(t *testing.T) {
	base := tempDir(t)
	dir := filepath.Join(base, "store")

	// An id generator that returns a path-traversal payload.
	evilID := "../../escaped"
	idGen := enginefile.IDGeneratorFunc[string](func(_ []string) (string, error) {
		return evilID, nil
	})

	repo, err := enginefile.New[Pet, string](idGen,
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
		enginefile.WithDirectoryMode(),
	)
	require.NoError(t, err)

	_, err = repo.Create(context.Background(), Pet{Name: "Rex", Kind: "dog"})
	require.Error(t, err, "a traversal id must be rejected, not written")

	// Nothing must have been written outside the storage directory.
	_, statErr := os.Stat(filepath.Join(base, "escaped.json"))
	assert.True(t, os.IsNotExist(statErr), "no file should escape the storage dir")
}

// TestDirectoryMode_SaveIsNonDestructive verifies that deletions clean up stale
// files and that a save never removes data before the new state is written (C5).
func TestDirectoryMode_SaveIsNonDestructive(t *testing.T) {
	dir := tempDir(t)
	repo, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
		enginefile.WithDirectoryMode(),
	)
	require.NoError(t, err)
	ctx := context.Background()

	for _, name := range []string{"Berlin", "Munich", "Hamburg"} {
		_, err = repo.Create(ctx, City{Name: name, Country: "Germany"})
		require.NoError(t, err)
	}

	citiesDir := filepath.Join(dir, "cities")
	entries, err := os.ReadDir(citiesDir)
	require.NoError(t, err)
	assert.Len(t, entries, 3, "three entity files expected, no leftover temp files")

	// Deleting one entity must remove exactly its file and keep the others.
	require.NoError(t, repo.Delete(ctx, 2))

	_, statErr := os.Stat(filepath.Join(citiesDir, "2.json"))
	assert.True(t, os.IsNotExist(statErr), "deleted entity file should be gone")
	for _, id := range []int{1, 3} {
		_, statErr := os.Stat(filepath.Join(citiesDir, fmt.Sprintf("%d.json", id)))
		require.NoError(t, statErr, "surviving entity %d must still be on disk", id)
	}

	remaining, total, err := repo.List(ctx, r3.Query{Pagination: r3.NoPagination()})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, remaining, 2)
}

// --------------------------------------------------------------------------
// YAML codec tests
// --------------------------------------------------------------------------

func TestYAML_CRUD(t *testing.T) {
	repo := newYAMLRepo[City, int](t, enginefile.IncrementIDGen[int]())
	ctx := context.Background()

	created, err := repo.Create(ctx, City{Name: "Tokyo", Country: "Japan"})
	require.NoError(t, err)
	assert.Equal(t, 1, created.ID)

	got, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Tokyo", got.Name)

	cities, total, err := repo.List(ctx, r3.Query{Pagination: r3.NoPagination()})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, cities, 1)
}

// --------------------------------------------------------------------------
// WithFilePath test
// --------------------------------------------------------------------------

func TestWithFilePath(t *testing.T) {
	dir := tempDir(t)
	customPath := filepath.Join(dir, "custom", "my-cities.json")

	repo, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithCodec(enginefile.JSONCodec()),
		enginefile.WithFilePath(customPath),
	)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)

	_, err = os.Stat(customPath)
	assert.NoError(t, err)
}

// --------------------------------------------------------------------------
// Data persistence test
// --------------------------------------------------------------------------

func TestPersistence(t *testing.T) {
	dir := tempDir(t)
	ctx := context.Background()

	// Create a repo and add data
	repo1, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)
	_, _ = repo1.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	_, _ = repo1.Create(ctx, City{Name: "Paris", Country: "France"})

	// Create a new repo instance pointing to the same directory
	repo2, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)

	// Data should persist
	cities, total, err := repo2.List(ctx, r3.Query{Pagination: r3.NoPagination()})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, cities, 2)

	// New ID should auto-increment correctly
	city3, err := repo2.Create(ctx, City{Name: "Tokyo", Country: "Japan"})
	require.NoError(t, err)
	assert.Equal(t, 3, city3.ID)
}

// --------------------------------------------------------------------------
// Custom codec test
// --------------------------------------------------------------------------

func TestCustomCodec(t *testing.T) {
	dir := tempDir(t)

	// Create a custom codec that wraps stdlib JSON with a custom extension
	codec := enginefile.NewCodec(".custom",
		func(w io.Writer) enginefile.Encoder {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "\t") // use tabs instead of spaces
			return enc
		},
		func(r io.Reader) enginefile.Decoder {
			return json.NewDecoder(r)
		},
	)

	repo, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(codec),
	)
	require.NoError(t, err)
	ctx := context.Background()

	_, err = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)

	// Verify the file exists with the custom extension
	_, err = os.Stat(filepath.Join(dir, "cities.custom"))
	require.NoError(t, err)

	// Verify we can read back
	got, err := repo.Get(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Berlin", got.Name)
}

// --------------------------------------------------------------------------
// Config tests
// --------------------------------------------------------------------------

func TestConfig_CustomPageSize(t *testing.T) {
	dir := tempDir(t)
	cfg := r3.DefaultConfig()
	cfg.Defaults.PageSize = 3

	repo, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
		enginefile.WithR3Options(r3.WithConfig(cfg)),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Create 5 cities
	for i := range 5 {
		_, err := repo.Create(ctx, City{Name: "City" + string(rune('A'+i)), Country: "X"})
		require.NoError(t, err)
	}

	// List without explicit pagination — should use config's PageSize=3
	entities, total, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total) // total count is all 5
	assert.Len(t, entities, 3)       // but only 3 returned (page size)
}

func TestConfig_DefaultPageSizeUnchanged(t *testing.T) {
	dir := tempDir(t)

	// No config override — default page size (50) applies
	repo, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Create 5 cities
	for i := range 5 {
		_, err := repo.Create(ctx, City{Name: "City" + string(rune('A'+i)), Country: "X"})
		require.NoError(t, err)
	}

	// List without explicit pagination — default 50, no pagination needed for 5 items
	entities, total, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, entities, 5)
}

// --------------------------------------------------------------------------
// Querier tests
// --------------------------------------------------------------------------

func TestNewQuerier_ReadOnly(t *testing.T) {
	dir := tempDir(t)
	ctx := context.Background()

	// Create a full CRUD to seed data
	repo, err := enginefile.New[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)

	_, err = repo.Create(ctx, City{Name: "Berlin", Country: "Germany"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, City{Name: "Paris", Country: "France"})
	require.NoError(t, err)

	// Create a Querier pointing to the same data
	querier, err := enginefile.NewQuerier[City, int](
		enginefile.IncrementIDGen[int](),
		enginefile.WithBaseDir(dir),
		enginefile.WithCodec(enginefile.JSONCodec()),
	)
	require.NoError(t, err)

	// Get works
	city, err := querier.Get(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Berlin", city.Name)

	// List works
	cities, total, err := querier.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, cities, 2)

	// The querier variable is typed as r3.Querier — calling Create/Update/Patch/Delete
	// on it is a compile error. This test verifies the runtime read path works.
}

// --------------------------------------------------------------------------
// Compile-time interface checks
// --------------------------------------------------------------------------

var (
	_ r3.CRUD[City, int]      = (*enginefile.BaseCRUD[City, int])(nil)
	_ r3.Querier[City, int]   = (*enginefile.BaseCRUD[City, int])(nil)
	_ r3.Commander[City, int] = (*enginefile.BaseCRUD[City, int])(nil)
)

// codecCity declares a value codec the file engine does not apply yet.
type codecCity struct {
	ID        int       `r3:"id,pk"`
	Name      string    `r3:"name"`
	StartedAt time.Time `r3:"started_at,codec:unixtime"`
}

// TestNew_CodecGuard verifies the construction guard fires: a backend that does
// not apply codecs must reject a declared codec loudly, not store it un-encoded.
func TestNew_CodecGuard(t *testing.T) {
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_, _ = enginefile.New[codecCity, int](
			enginefile.IDGeneratorFunc[int](func(_ []int) (int, error) { return 1, nil }),
			enginefile.WithBaseDir(t.TempDir()),
		)
	}()
	require.NotNil(t, recovered, "construction with a codec model must panic")
	err, ok := recovered.(error)
	require.True(t, ok)
	require.ErrorIs(t, err, r3.ErrCodecNotSupported)
}
