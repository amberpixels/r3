package enginemongo_test

import (
	"slices"
	"testing"
	"time"

	"github.com/expectto/be"

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

	be.RequireThat(t, meta.SoftDeleteField, be.Eq("deleted_at"))
	be.RequireThat(t, meta.SoftDeleteZero, be.NotNil(),
		"SoftDeleteZero is nil for a non-pointer time.Time field; want the zero time")

	z, ok := meta.SoftDeleteZero.(time.Time)
	be.RequireThat(t, ok && z.IsZero(), be.True(),
		"SoftDeleteZero = %v, want zero time.Time", meta.SoftDeleteZero)
}

// TestSoftDeleteZero_Pointer verifies a pointer soft-delete field captures no
// zero (live records are stored as null, matched by {$eq: nil}).
func TestSoftDeleteZero_Pointer(t *testing.T) {
	meta := enginemongo.GetStructMeta[ptrSoftDelete]()

	be.RequireThat(t, meta.SoftDeleteField, be.Eq("deleted_at"))
	be.RequireThat(t, meta.SoftDeleteZero, be.Nil(),
		"SoftDeleteZero = %v, want nil for a pointer field", meta.SoftDeleteZero)
}

// docStep and docAddress are ordinary nested values, not entities of their own.
type docStep struct {
	Kind string `bson:"kind"`
	Reps int    `bson:"reps"`
}

type docAddress struct {
	City string `bson:"city"`
}

// docFields is the document shape the old type-based gate discarded wholesale:
// value slices, a map and a subdocument, none of which declares a relation. In a
// document store each is a native value, so each is a stored field.
type docFields struct {
	ID      string            `bson:"_id"     r3:"pk"`
	Name    string            `bson:"name"`
	Tags    []string          `bson:"tags"`
	Steps   []docStep         `bson:"steps"`
	Labels  map[string]string `bson:"labels"`
	Address docAddress        `bson:"address"`
	Owner   *docAddress       `bson:"owner"`
	Skipped []string          `bson:"-"`
	Hidden  []string          `               r3:"-"`
}

// TestStructMeta_UntaggedValueFieldsAreStored pins GH-22: an untagged slice, map
// or subdocument reaches Fields, so ToBSONDoc writes it. It used to be dropped on
// write while the bson decoder still populated it on read, so the type
// round-tripped one way only.
func TestStructMeta_UntaggedValueFieldsAreStored(t *testing.T) {
	meta := enginemongo.GetStructMeta[docFields]()

	for _, name := range []string{"_id", "name", "tags", "steps", "labels", "address", "owner"} {
		be.AssertThat(t, slices.Contains(meta.Fields, name), be.True(),
			"field %q missing from meta.Fields %v", name, meta.Fields)
	}

	for _, name := range []string{"skipped", "hidden"} {
		be.AssertThat(t, slices.Contains(meta.Fields, name), be.False(),
			"explicitly skipped field %q is in meta.Fields %v", name, meta.Fields)
	}

	be.AssertThat(t, len(meta.Relations), be.Eq(0))
}

// TestToBSONDoc_CarriesValueSlices is the write half: the values, not just the
// names, have to reach the document.
func TestToBSONDoc_CarriesValueSlices(t *testing.T) {
	meta := enginemongo.GetStructMeta[docFields]()

	doc := meta.ToBSONDoc(docFields{
		ID:      "c1",
		Name:    "Kyiv",
		Tags:    []string{"capital", "river"},
		Steps:   []docStep{{Kind: "warmup", Reps: 3}},
		Labels:  map[string]string{"tier": "a"},
		Address: docAddress{City: "Kyiv"},
	}, true)

	be.AssertThat(t, doc["tags"], be.Eq([]string{"capital", "river"}))
	be.AssertThat(t, doc["steps"], be.Eq([]docStep{{Kind: "warmup", Reps: 3}}))
	be.AssertThat(t, doc["labels"], be.Eq(map[string]string{"tier": "a"}))
	be.AssertThat(t, doc["address"], be.Eq(docAddress{City: "Kyiv"}))
}

type relItem struct {
	ID      string `bson:"_id"      r3:"pk"`
	OwnerID string `bson:"owner_id"`
}

// relOwner declares its relation the one way a document store recognises one.
type relOwner struct {
	ID    string    `bson:"_id"  r3:"pk"`
	Name  string    `bson:"name"`
	Items []relItem `            r3:"rel:has-many,fk:owner_id"`
}

type gormChild struct {
	ID      string `bson:"_id"      r3:"pk"`
	OwnerID string `bson:"owner_id"`
}

// gormOwner is a model shared with the GORM driver: its association is declared
// through the gorm tag alone, which keeps it out of the document too.
type gormOwner struct {
	ID       string      `bson:"_id"  r3:"pk"`
	Name     string      `bson:"name"`
	Children []gormChild `                    gorm:"foreignKey:OwnerID"`
}

// TestStructMeta_DeclaredRelationsStayRelations is the other side of GH-22: a
// declared relation is still a relation, never a stored array.
func TestStructMeta_DeclaredRelationsStayRelations(t *testing.T) {
	t.Run("r3 rel tag", func(t *testing.T) {
		meta := enginemongo.GetStructMeta[relOwner]()

		be.RequireThat(t, len(meta.Relations), be.Eq(1))
		be.AssertThat(t, meta.Relations[0].FieldName, be.Eq("Items"))
		be.AssertThat(t, slices.Contains(meta.Fields, "items"), be.False(),
			"a declared relation was stored as a field: %v", meta.Fields)
	})

	t.Run("gorm association tag", func(t *testing.T) {
		meta := enginemongo.GetStructMeta[gormOwner]()

		be.AssertThat(t, slices.Contains(meta.Fields, "children"), be.False(),
			"a gorm association was stored as a field: %v", meta.Fields)
	})
}

// cycleAuthor and cycleBook declare relations pointing at each other, the shape
// that makes "ask buildRelationMeta whether this is a relation" recurse forever:
// building either one's meta walks into the other's relations and back.
type cycleAuthor struct {
	ID    string      `bson:"_id"  r3:"pk"`
	Name  string      `bson:"name"`
	Books []cycleBook `            r3:"rel:has-many,fk:author_id"`
}

type cycleBook struct {
	ID       string       `bson:"_id"       r3:"pk"`
	AuthorID string       `bson:"author_id"`
	Author   *cycleAuthor `                 r3:"rel:belongs-to,fk:author_id"`
}

// TestStructMeta_MutualRelationsTerminate pins the recursion stop: relation
// metadata is built one level deep, so a mutual relation resolves instead of
// overflowing the stack at construction.
func TestStructMeta_MutualRelationsTerminate(t *testing.T) {
	meta := enginemongo.GetStructMeta[cycleAuthor]()

	be.RequireThat(t, len(meta.Relations), be.Eq(1))
	be.AssertThat(t, meta.Relations[0].FieldName, be.Eq("Books"))
	be.AssertThat(t, meta.Relations[0].TargetMeta.CollectionName, be.Eq("cycle_books"))
	be.AssertThat(t, len(meta.Relations[0].TargetMeta.Relations), be.Eq(0),
		"relation targets must not carry their own relations")
}
