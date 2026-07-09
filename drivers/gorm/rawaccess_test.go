package r3gorm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/amberpixels/r3"
	r3gorm "github.com/amberpixels/r3/drivers/gorm"
	enginefile "github.com/amberpixels/r3/engine/file"
	"github.com/amberpixels/r3/features/permissions"
)

func TestRawOf_BareRepo(t *testing.T) {
	db := setupRelDB(t)
	base := r3gorm.NewGormCRUD[relSquad, int64](db)
	var repo r3.CRUD[relSquad, int64] = base

	raw, err := r3gorm.RawOf(repo)
	if err != nil {
		t.Fatalf("RawOf: %v", err)
	}
	if raw == nil {
		t.Fatal("RawOf returned nil builder")
	}

	gdb, err := r3gorm.DBOf(repo)
	if err != nil {
		t.Fatalf("DBOf: %v", err)
	}
	if gdb == nil {
		t.Fatal("DBOf returned nil *gorm.DB")
	}
}

func TestRawOf_ThroughDecorator(t *testing.T) {
	db := setupRelDB(t)
	base := r3gorm.NewGormCRUD[relSquad, int64](db)

	// Wrap in a permissions decorator: RawOf/DBOf must still unwrap to the gorm
	// base (via r3.As following Unwrap).
	var repo r3.CRUD[relSquad, int64] = permissions.WithPermissions[relSquad, int64](
		base, permissions.AllowAll[relSquad, int64](),
	)

	raw, err := r3gorm.RawOf(repo)
	if err != nil {
		t.Fatalf("RawOf through decorator: %v", err)
	}
	if raw == nil {
		t.Fatal("RawOf returned nil builder through decorator")
	}

	if _, err := r3gorm.DBOf(repo); err != nil {
		t.Fatalf("DBOf through decorator: %v", err)
	}
}

func TestDBOf_ThroughDecorator_QueryWorks(t *testing.T) {
	db := setupRelDB(t)
	seedRelData(t, db)
	base := r3gorm.NewGormCRUD[relSquad, int64](db)
	var repo r3.CRUD[relSquad, int64] = permissions.WithPermissions[relSquad, int64](
		base, permissions.AllowAll[relSquad, int64](),
	)

	// The *gorm.DB unwrapped from behind the decorator actually reaches the DB.
	gdb, err := r3gorm.DBOf(repo)
	if err != nil {
		t.Fatalf("DBOf: %v", err)
	}
	var n int64
	if err := gdb.WithContext(context.Background()).Table("rel_squads").Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Fatalf("count = %d, want 3", n)
	}
}

func TestRawOf_NonGormRepo(t *testing.T) {
	repo, err := enginefile.New[relSquad, int64](
		enginefile.IncrementIDGen[int64](),
		enginefile.WithBaseDir(t.TempDir()),
	)
	if err != nil {
		t.Fatalf("new file repo: %v", err)
	}

	if _, err := r3gorm.RawOf[relSquad, int64](repo); !errors.Is(err, r3gorm.ErrRawNotSupported) {
		t.Fatalf("RawOf on file repo: got %v, want ErrRawNotSupported", err)
	}
	if _, err := r3gorm.DBOf[relSquad, int64](repo); !errors.Is(err, r3gorm.ErrRawNotSupported) {
		t.Fatalf("DBOf on file repo: got %v, want ErrRawNotSupported", err)
	}
}
