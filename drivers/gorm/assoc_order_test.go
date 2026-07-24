package r3gorm_test

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
)

// Models exercising the M2M order column (`order:sort_order`): an album whose
// photo order is part of the data.

type ordPhoto struct {
	ID   int64 `gorm:"primarykey"`
	Name string
}

func (ordPhoto) TableName() string { return "ord_photos" }

type ordAlbum struct {
	ID     int64 `gorm:"primarykey"`
	Name   string
	Photos []ordPhoto `gorm:"-"          r3:"rel:many-to-many,join:ord_album_photos,fk:album_id,ref:photo_id,order:sort_order"`
}

func (ordAlbum) TableName() string { return "ord_albums" }

func setupOrdDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.AutoMigrate(&ordPhoto{}, &ordAlbum{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(
		`CREATE TABLE ord_album_photos (album_id INTEGER NOT NULL, photo_id INTEGER NOT NULL,
		 sort_order INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (album_id, photo_id))`,
	).Error; err != nil {
		t.Fatalf("create join: %v", err)
	}
	return db
}

func photoIDs(photos []ordPhoto) []int64 {
	ids := make([]int64, len(photos))
	for i, p := range photos {
		ids[i] = p.ID
	}
	return ids
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestM2MOrderColumn_RoundTrip: the relation slice's order survives
// create → preload and reorder-update → preload, via the join table's
// order column.
func TestM2MOrderColumn_RoundTrip(t *testing.T) {
	db := setupOrdDB(t)
	photos := []ordPhoto{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}, {ID: 3, Name: "c"}}
	if err := db.Create(&photos).Error; err != nil {
		t.Fatalf("seed photos: %v", err)
	}

	albums := r3gorm.NewGormCRUD[ordAlbum, int64](db)
	ctx := context.Background()
	preload := r3.Query{Preloads: r3.Preloads{r3.NewPreloadSpec("Photos")}}

	// Create in a non-monotonic order: {3, 1, 2}.
	created, err := albums.Create(ctx, ordAlbum{
		ID: 1, Name: "A",
		Photos: []ordPhoto{{ID: 3}, {ID: 1}, {ID: 2}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := albums.Get(ctx, created.ID, preload)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if want := []int64{3, 1, 2}; !equalIDs(photoIDs(got.Photos), want) {
		t.Fatalf("created order: got %v, want %v", photoIDs(got.Photos), want)
	}

	// Reorder to {2, 3, 1} and update.
	got.Photos = []ordPhoto{{ID: 2}, {ID: 3}, {ID: 1}}
	if _, err := albums.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err = albums.Get(ctx, created.ID, preload)
	if err != nil {
		t.Fatalf("get after reorder: %v", err)
	}
	if want := []int64{2, 3, 1}; !equalIDs(photoIDs(got.Photos), want) {
		t.Fatalf("reordered: got %v, want %v", photoIDs(got.Photos), want)
	}

	// The order column itself carries the slice indexes (not just rowid luck).
	var orders []int
	if err := db.Raw(
		"SELECT sort_order FROM ord_album_photos WHERE album_id = 1 ORDER BY photo_id",
	).Scan(&orders).Error; err != nil {
		t.Fatalf("read join: %v", err)
	}
	// photo_id 1 → index 2, photo_id 2 → index 0, photo_id 3 → index 1.
	if want := []int{
		2,
		0,
		1,
	}; len(orders) != 3 || orders[0] != want[0] || orders[1] != want[1] ||
		orders[2] != want[2] {
		t.Fatalf("sort_order values: got %v, want %v", orders, want)
	}
}
