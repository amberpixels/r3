package history_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	"github.com/amberpixels/r3/features/history"
	"github.com/expectto/be"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A pet with the three relation shapes history records: an ordered M2M (photo
// order is part of the data), an unordered M2M, and an owned has-many whose
// children the driver re-inserts on every save.
type PetPhoto struct {
	ID  int64  `r3:"id,pk"`
	URL string `r3:"url"`
}

type PetTag struct {
	ID   int64  `r3:"id,pk"`
	Name string `r3:"name"`
}

type Vaccination struct {
	ID      int64  `r3:"id,pk"`
	PetID   int64  `r3:"pet_id"`
	Vaccine string `r3:"vaccine"`
}

type Pet struct {
	ID           int64         `r3:"id,pk"`
	Name         string        `r3:"name"`
	Photos       []PetPhoto    `r3:"rel:many-to-many,join:pet_photo_links,fk:pet_id,ref:photo_id,order:sort_order" gorm:"-"`
	Tags         []PetTag      `r3:"rel:many-to-many,join:pet_tag_links,fk:pet_id,ref:tag_id"                      gorm:"-"`
	Vaccinations []Vaccination `r3:"rel:has-many,fk:pet_id,owned"                                                  gorm:"-"`
}

type petFixture struct {
	repo   *history.CRUD[Pet, int64]
	store  r3.CRUD[history.ChangeRecord, string]
	photos []PetPhoto
	tags   []PetTag
}

// setupPetRepo backs both the pets and their history with gorm on sqlite, so
// recorded member lists go through a real store round-trip before replay.
func setupPetRepo(t *testing.T, defaultGet ...r3.Query) petFixture {
	t.Helper()
	ctx := context.Background()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	be.NoError(t, err)
	sqlDB, err := db.DB()
	be.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	be.NoError(t, db.AutoMigrate(&PetPhoto{}, &PetTag{}, &Vaccination{}, &Pet{}, &history.ChangeRecord{}))
	be.NoError(t, db.Exec(`CREATE TABLE pet_photo_links (pet_id INTEGER NOT NULL, photo_id INTEGER NOT NULL,
		sort_order INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (pet_id, photo_id))`).Error)
	be.NoError(t, db.Exec(`CREATE TABLE pet_tag_links (pet_id INTEGER NOT NULL, tag_id INTEGER NOT NULL,
		PRIMARY KEY (pet_id, tag_id))`).Error)

	f := petFixture{store: r3gorm.NewGormCRUD[history.ChangeRecord, string](db)}
	for _, url := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		p := PetPhoto{URL: url}
		be.NoError(t, db.WithContext(ctx).Create(&p).Error)
		f.photos = append(f.photos, p)
	}
	for _, name := range []string{"calm", "playful"} {
		tg := PetTag{Name: name}
		be.NoError(t, db.WithContext(ctx).Create(&tg).Error)
		f.tags = append(f.tags, tg)
	}

	pets := r3gorm.NewGormCRUD[Pet, int64](db)
	for _, q := range defaultGet {
		pets.SetDefaultGetQuery(q)
	}
	f.repo = history.WithHistory[Pet, int64](
		pets, f.store,
		history.WithIDFunc[Pet, int64](func(p Pet) int64 { return p.ID }),
	)
	return f
}

func (f petFixture) records(t *testing.T, id int64) []history.ChangeRecord {
	t.Helper()
	recs, _, err := f.store.List(context.Background(), history.QueryForRecord("pets", strconv.FormatInt(id, 10)))
	be.NoError(t, err)
	return recs
}

// latestChanges returns the newest record's changes by field, updated_at aside.
func (f petFixture) latestChanges(t *testing.T, id int64) map[string]history.FieldChange {
	t.Helper()
	recs := f.records(t, id)
	be.RequireThat(t, len(recs), be.Gt(0))
	out := map[string]history.FieldChange{}
	for _, c := range recs[len(recs)-1].Changes.Val {
		out[c.Field] = c
	}
	return out
}

// ids reads a recorded member list after the store round-trip (JSON numbers).
func ids(t *testing.T, v any) []int64 {
	t.Helper()
	list, ok := v.([]any)
	be.RequireThat(t, ok, be.True())
	out := make([]int64, len(list))
	for i, m := range list {
		n, ok := m.(float64)
		be.RequireThat(t, ok, be.True())
		out[i] = int64(n)
	}
	return out
}

func photoIDsOf(photos []PetPhoto) []int64 {
	out := make([]int64, len(photos))
	for i, p := range photos {
		out[i] = p.ID
	}
	return out
}

func TestRelationDiff_M2MAddRemoveReorder(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()
	a, b, c := f.photos[0], f.photos[1], f.photos[2]

	pet, err := f.repo.Create(ctx, Pet{Name: "Rex", Photos: []PetPhoto{a, b}})
	be.NoError(t, err)

	created := f.latestChanges(t, pet.ID)
	be.AssertThat(t, created["photos"].OldValue, be.Nil())
	be.AssertThat(t, ids(t, created["photos"].NewValue), be.Eq([]int64{a.ID, b.ID}))

	// Remove a, add c, and put c first: one save, one photos change.
	_, err = f.repo.Update(ctx, Pet{ID: pet.ID, Name: "Rex", Photos: []PetPhoto{c, b}})
	be.NoError(t, err)

	changes := f.latestChanges(t, pet.ID)
	be.AssertThat(t, ids(t, changes["photos"].OldValue), be.Eq([]int64{a.ID, b.ID}))
	be.AssertThat(t, ids(t, changes["photos"].NewValue), be.Eq([]int64{c.ID, b.ID}))
	be.AssertThat(t, len(changes), be.Eq(1))
}

func TestRelationDiff_ReorderCountsOnlyWhenOrderIsPersisted(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()
	a, b := f.photos[0], f.photos[1]
	calm, playful := f.tags[0], f.tags[1]

	pet, err := f.repo.Create(ctx, Pet{
		Name: "Rex", Photos: []PetPhoto{a, b}, Tags: []PetTag{calm, playful},
	})
	be.NoError(t, err)

	_, err = f.repo.Update(ctx, Pet{
		ID: pet.ID, Name: "Rex", Photos: []PetPhoto{b, a}, Tags: []PetTag{playful, calm},
	})
	be.NoError(t, err)

	changes := f.latestChanges(t, pet.ID)
	be.AssertThat(t, ids(t, changes["photos"].NewValue), be.Eq([]int64{b.ID, a.ID}))
	_, tagsChanged := changes["tags"]
	be.AssertThat(t, tagsChanged, be.False())
}

func TestRelationDiff_NilRelationRecordsNothing(t *testing.T) {
	// The default Get preloads photos, so the old side is loaded; the new side is
	// nil ("not loaded") and must not read as a clear.
	f := setupPetRepo(t, r3.Query{Preloads: r3.Preloads{r3.NewPreloadSpec("Photos")}})
	ctx := context.Background()

	pet, err := f.repo.Create(ctx, Pet{Name: "Rex", Photos: []PetPhoto{f.photos[0]}})
	be.NoError(t, err)

	_, err = f.repo.Update(ctx, Pet{ID: pet.ID, Name: "Max"})
	be.NoError(t, err)

	changes := f.latestChanges(t, pet.ID)
	_, photosChanged := changes["photos"]
	be.AssertThat(t, photosChanged, be.False())
	be.AssertThat(t, changes["name"].NewValue, be.Eq(any("Max")))

	got, err := f.repo.Get(ctx, pet.ID)
	be.NoError(t, err)
	be.AssertThat(t, photoIDsOf(got.Photos), be.Eq([]int64{f.photos[0].ID}))
}

func TestRelationDiff_EmptyRelationRecordsClear(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()
	a, b := f.photos[0], f.photos[1]

	pet, err := f.repo.Create(ctx, Pet{Name: "Rex", Photos: []PetPhoto{a, b}})
	be.NoError(t, err)

	_, err = f.repo.Update(ctx, Pet{ID: pet.ID, Name: "Rex", Photos: []PetPhoto{}})
	be.NoError(t, err)

	changes := f.latestChanges(t, pet.ID)
	be.AssertThat(t, ids(t, changes["photos"].OldValue), be.Eq([]int64{a.ID, b.ID}))
	be.AssertThat(t, ids(t, changes["photos"].NewValue), be.Eq([]int64{}))
}

func TestRelationDiff_FirstAttachFromEmpty(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()

	pet, err := f.repo.Create(ctx, Pet{Name: "Rex"})
	be.NoError(t, err)

	_, err = f.repo.Update(ctx, Pet{ID: pet.ID, Name: "Rex", Photos: []PetPhoto{f.photos[0]}})
	be.NoError(t, err)

	changes := f.latestChanges(t, pet.ID)
	be.AssertThat(t, ids(t, changes["photos"].OldValue), be.Eq([]int64{}))
	be.AssertThat(t, ids(t, changes["photos"].NewValue), be.Eq([]int64{f.photos[0].ID}))
}

func TestRelationDiff_UnchangedRelationRecordsNothing(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()

	pet, err := f.repo.Create(ctx, Pet{
		Name:         "Rex",
		Photos:       []PetPhoto{f.photos[0]},
		Vaccinations: []Vaccination{{Vaccine: "rabies"}},
	})
	be.NoError(t, err)

	// The vaccinations are re-inserted with new PKs; that alone is no change.
	_, err = f.repo.Update(ctx, Pet{
		ID:           pet.ID,
		Name:         "Rex",
		Photos:       []PetPhoto{f.photos[0]},
		Vaccinations: []Vaccination{{Vaccine: "rabies"}},
	})
	be.NoError(t, err)

	recs := f.records(t, pet.ID)
	be.RequireThat(t, len(recs), be.Eq(2))
	be.AssertThat(t, len(recs[1].Changes.Val), be.Eq(0))
}

func TestRelationDiff_OwnedHasManyByValue(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()

	pet, err := f.repo.Create(ctx, Pet{Name: "Rex", Vaccinations: []Vaccination{{Vaccine: "rabies"}}})
	be.NoError(t, err)

	_, err = f.repo.Update(ctx, Pet{
		ID: pet.ID, Name: "Rex",
		Vaccinations: []Vaccination{{Vaccine: "rabies"}, {Vaccine: "parvo"}},
	})
	be.NoError(t, err)

	changes := f.latestChanges(t, pet.ID)
	be.AssertThat(t, changes["vaccinations"].OldValue, be.Eq(any([]any{
		map[string]any{"vaccine": "rabies"},
	})))
	be.AssertThat(t, changes["vaccinations"].NewValue, be.Eq(any([]any{
		map[string]any{"vaccine": "rabies"},
		map[string]any{"vaccine": "parvo"},
	})))
}

func TestRelationDiff_RevertRestoresMembership(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()
	a, b, c := f.photos[0], f.photos[1], f.photos[2]

	pet, err := f.repo.Create(ctx, Pet{
		Name: "Rex", Photos: []PetPhoto{a, b}, Vaccinations: []Vaccination{{Vaccine: "rabies"}},
	})
	be.NoError(t, err)
	_, err = f.repo.Update(ctx, Pet{
		ID: pet.ID, Name: "Rex", Photos: []PetPhoto{c}, Vaccinations: []Vaccination{},
	})
	be.NoError(t, err)

	_, err = f.repo.Reverter().RevertTo(ctx, pet.ID, 1)
	be.NoError(t, err)

	got, err := f.repo.Get(ctx, pet.ID, r3.Query{Preloads: r3.Preloads{
		r3.NewPreloadSpec("Photos"), r3.NewPreloadSpec("Vaccinations"),
	}})
	be.NoError(t, err)
	be.AssertThat(t, photoIDsOf(got.Photos), be.Eq([]int64{a.ID, b.ID}))
	be.RequireThat(t, len(got.Vaccinations), be.Eq(1))
	be.AssertThat(t, got.Vaccinations[0].Vaccine, be.Eq("rabies"))

	// The revert's own record carries the relation change back.
	changes := f.latestChanges(t, pet.ID)
	be.AssertThat(t, ids(t, changes["photos"].NewValue), be.Eq([]int64{a.ID, b.ID}))
}

func TestRelationDiff_RevertBeforeRelationWasRecordedLeavesIt(t *testing.T) {
	f := setupPetRepo(t)
	ctx := context.Background()

	pet, err := f.repo.Create(ctx, Pet{Name: "Rex"})
	be.NoError(t, err)
	_, err = f.repo.Update(ctx, Pet{ID: pet.ID, Name: "Max", Photos: []PetPhoto{f.photos[0]}})
	be.NoError(t, err)

	// Version 1 never recorded photos, so reverting to it restores the name and
	// leaves the photos as they are rather than clearing them.
	_, err = f.repo.Reverter().RevertTo(ctx, pet.ID, 1)
	be.NoError(t, err)

	got, err := f.repo.Get(ctx, pet.ID, r3.Query{Preloads: r3.Preloads{r3.NewPreloadSpec("Photos")}})
	be.NoError(t, err)
	be.AssertThat(t, got.Name, be.Eq("Rex"))
	be.AssertThat(t, photoIDsOf(got.Photos), be.Eq([]int64{f.photos[0].ID}))
}
