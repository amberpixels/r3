package history_test

import (
	"testing"
	"time"

	"github.com/amberpixels/r3/features/history"
)

// Test entity types

type SimpleEntity struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

type TaggedEntity struct {
	ID     int64  `r3:"id,pk"`
	Title  string `r3:"title"`
	Status string `           db:"status"`
}

type NestedAddress struct {
	Street string `db:"street"`
	City   string `db:"city"`
	Zip    string `db:"zip"`
}

type EntityWithNested struct {
	ID      int64         `db:"id,pk"`
	Name    string        `db:"name"`
	Address NestedAddress `db:"address"`
}

type EntityWithPointer struct {
	ID    int64   `db:"id,pk"`
	Name  string  `db:"name"`
	Score *int    `db:"score"`
	Note  *string `db:"note"`
}

type EntityWithTime struct {
	ID        int64     `db:"id,pk"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

type EntityWithSlice struct {
	ID    int64    `db:"id,pk"`
	Name  string   `db:"name"`
	Tags  []string `db:"tags"` // should be skipped (relation-like)
	Items []int    `db:"-"`    // explicitly skipped
}

type EntityWithBSON struct {
	ID   string `bson:"_id"`
	Name string `bson:"name"`
	Age  int    `bson:"age"`
}

func TestDiff_NoChanges(t *testing.T) {
	old := SimpleEntity{ID: 1, Name: "Alice", Age: 30}
	cur := SimpleEntity{ID: 1, Name: "Alice", Age: 30}

	changes := history.Diff(old, cur)
	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d: %v", len(changes), changes)
	}
}

func TestDiff_ScalarChanges(t *testing.T) {
	old := SimpleEntity{ID: 1, Name: "Alice", Age: 30}
	cur := SimpleEntity{ID: 1, Name: "Bob", Age: 25}

	changes := history.Diff(old, cur)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	// Check name change
	assertChange(t, changes, "name")
	assertChange(t, changes, "age")
}

func TestDiff_IDChangeTracked(t *testing.T) {
	// Even PK changes should be tracked in the diff (the diff doesn't know about PKs)
	old := SimpleEntity{ID: 1, Name: "Alice", Age: 30}
	cur := SimpleEntity{ID: 2, Name: "Alice", Age: 30}

	changes := history.Diff(old, cur)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	assertChange(t, changes, "id")
}

func TestDiff_R3Tags(t *testing.T) {
	old := TaggedEntity{ID: 1, Title: "Old", Status: "draft"}
	cur := TaggedEntity{ID: 1, Title: "New", Status: "published"}

	changes := history.Diff(old, cur)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	assertChange(t, changes, "title")
	assertChange(t, changes, "status")
}

func TestDiff_NestedStruct_DotNotation(t *testing.T) {
	old := EntityWithNested{
		ID:      1,
		Name:    "Alice",
		Address: NestedAddress{Street: "123 Main", City: "NYC", Zip: "10001"},
	}
	cur := EntityWithNested{
		ID:      1,
		Name:    "Alice",
		Address: NestedAddress{Street: "456 Oak", City: "NYC", Zip: "10002"},
	}

	changes := history.Diff(old, cur)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	assertChange(t, changes, "address.street")
	assertChange(t, changes, "address.zip")
}

func TestDiff_PointerFields(t *testing.T) {
	score1 := 100
	score2 := 200
	note := "hello"

	old := EntityWithPointer{ID: 1, Name: "A", Score: &score1, Note: nil}
	cur := EntityWithPointer{ID: 1, Name: "A", Score: &score2, Note: &note}

	changes := history.Diff(old, cur)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}

	assertChange(t, changes, "score")
	assertChange(t, changes, "note")
}

func TestDiff_TimeField(t *testing.T) {
	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	old := EntityWithTime{ID: 1, Name: "A", CreatedAt: t1}
	cur := EntityWithTime{ID: 1, Name: "A", CreatedAt: t2}

	changes := history.Diff(old, cur)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}

	if changes[0].Field != "created_at" {
		t.Errorf("expected field 'created_at', got %q", changes[0].Field)
	}
}

func TestDiff_SkipsSliceFields(t *testing.T) {
	old := EntityWithSlice{ID: 1, Name: "A", Tags: []string{"a"}, Items: []int{1}}
	cur := EntityWithSlice{ID: 1, Name: "B", Tags: []string{"b"}, Items: []int{2}}

	changes := history.Diff(old, cur)
	// Only Name should show up, Tags and Items are slice types (skipped)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	assertChange(t, changes, "name")
}

func TestDiff_BSONTags(t *testing.T) {
	old := EntityWithBSON{ID: "abc", Name: "Old", Age: 20}
	cur := EntityWithBSON{ID: "abc", Name: "New", Age: 25}

	changes := history.Diff(old, cur)
	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d: %v", len(changes), changes)
	}
	assertChange(t, changes, "name")
	assertChange(t, changes, "age")
}

func TestDiffWithFields(t *testing.T) {
	old := SimpleEntity{ID: 1, Name: "Alice", Age: 30}
	cur := SimpleEntity{ID: 1, Name: "Bob", Age: 25}

	// Only ask for "name" changes
	changes := history.DiffWithFields(old, cur, []string{"name"})
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	assertChange(t, changes, "name")
}

func TestDiffCreate(t *testing.T) {
	entity := SimpleEntity{ID: 1, Name: "Alice", Age: 30}

	changes := history.DiffCreate(entity)
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d: %v", len(changes), changes)
	}

	for _, c := range changes {
		if c.OldValue != nil {
			t.Errorf("DiffCreate: field %s should have nil OldValue, got %v", c.Field, c.OldValue)
		}
		if c.NewValue == nil {
			t.Errorf("DiffCreate: field %s should have non-nil NewValue", c.Field)
		}
	}
}

func TestDiffDelete(t *testing.T) {
	entity := SimpleEntity{ID: 1, Name: "Alice", Age: 30}

	changes := history.DiffDelete(entity)
	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d: %v", len(changes), changes)
	}

	for _, c := range changes {
		if c.OldValue == nil {
			t.Errorf("DiffDelete: field %s should have non-nil OldValue", c.Field)
		}
		if c.NewValue != nil {
			t.Errorf("DiffDelete: field %s should have nil NewValue, got %v", c.Field, c.NewValue)
		}
	}
}

func TestResolveColumnName_Priority(t *testing.T) {
	// Test that r3 tag takes priority over db tag
	type Entity struct {
		Field1 string `r3:"r3_name" db:"db_name"`
		Field2 string `             db:"db_only"`
		Field3 string `                          bson:"bson_only"`
		Field4 string // no tags -> snake_case
	}

	old := Entity{Field1: "a", Field2: "b", Field3: "c", Field4: "d"}
	cur := Entity{Field1: "x", Field2: "y", Field3: "z", Field4: "w"}

	changes := history.Diff(old, cur)
	if len(changes) != 4 {
		t.Fatalf("expected 4 changes, got %d: %v", len(changes), changes)
	}

	fieldNames := make(map[string]bool)
	for _, c := range changes {
		fieldNames[c.Field] = true
	}

	expected := []string{"r3_name", "db_only", "bson_only", "field4"}
	for _, name := range expected {
		if !fieldNames[name] {
			t.Errorf("expected field %q in changes, got fields: %v", name, fieldNames)
		}
	}
}

func TestSnapshot_RoundTrip(t *testing.T) {
	entity := SimpleEntity{ID: 42, Name: "Test", Age: 99}

	data := history.MarshalSnapshot(entity)
	if data == nil {
		t.Fatal("MarshalSnapshot returned nil")
	}

	restored, err := history.UnmarshalSnapshot[SimpleEntity](data)
	if err != nil {
		t.Fatalf("UnmarshalSnapshot failed: %v", err)
	}

	if restored != entity {
		t.Errorf("round-trip mismatch: got %+v, want %+v", restored, entity)
	}
}

// assertChange checks that a FieldChange with the given field name exists.
func assertChange(t *testing.T, changes []history.FieldChange, field string) {
	t.Helper()
	for _, c := range changes {
		if c.Field == field {
			return // found it
		}
	}
	t.Errorf("expected change for field %q not found in %v", field, changes)
}
