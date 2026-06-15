package enginemongo_test

import (
	"testing"
	"time"

	enginemongo "github.com/amberpixels/r3/engine/mongo"
)

// nonPtrSoftDelete uses a non-pointer time.Time soft-delete field, which mongo
// persists as the zero BSON Date (not null) for never-deleted records.
type nonPtrSoftDelete struct {
	ID        string    `r3:"_id,pk"`
	Name      string    `r3:"name"`
	DeletedAt time.Time `r3:"deleted_at,soft_delete"`
}

// ptrSoftDelete uses a pointer soft-delete field, which mongo persists as null
// for never-deleted records.
type ptrSoftDelete struct {
	ID        string     `r3:"_id,pk"`
	Name      string     `r3:"name"`
	DeletedAt *time.Time `r3:"deleted_at,soft_delete"`
}

// TestSoftDeleteZero_NonPointer verifies M6: a non-pointer soft-delete field
// captures its zero value, so the "not deleted" filter can match live records
// (stored as the zero Date) in addition to null.
func TestSoftDeleteZero_NonPointer(t *testing.T) {
	meta := enginemongo.GetStructMeta[nonPtrSoftDelete]()

	if meta.SoftDeleteField != "deleted_at" {
		t.Fatalf("SoftDeleteField = %q, want %q", meta.SoftDeleteField, "deleted_at")
	}
	if meta.SoftDeleteZero == nil {
		t.Fatal("SoftDeleteZero is nil for a non-pointer time.Time field; want the zero time")
	}
	if z, ok := meta.SoftDeleteZero.(time.Time); !ok || !z.IsZero() {
		t.Fatalf("SoftDeleteZero = %v, want zero time.Time", meta.SoftDeleteZero)
	}
}

// TestSoftDeleteZero_Pointer verifies a pointer soft-delete field captures no
// zero (live records are stored as null, matched by {$eq: nil}).
func TestSoftDeleteZero_Pointer(t *testing.T) {
	meta := enginemongo.GetStructMeta[ptrSoftDelete]()

	if meta.SoftDeleteField != "deleted_at" {
		t.Fatalf("SoftDeleteField = %q, want %q", meta.SoftDeleteField, "deleted_at")
	}
	if meta.SoftDeleteZero != nil {
		t.Fatalf("SoftDeleteZero = %v, want nil for a pointer field", meta.SoftDeleteZero)
	}
}
