package r3_test

import (
	"encoding/json"
	"testing"

	"github.com/amberpixels/r3"
)

func TestJSONColumn_ValueAndScan(t *testing.T) {
	type Inner struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	original := r3.NewJSONColumn(Inner{Name: "Alice", Age: 30})

	// Value -> driver string
	val, err := original.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	str, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}

	// Scan from string
	var scanned r3.JSONColumn[Inner]
	if err := scanned.Scan(str); err != nil {
		t.Fatalf("Scan(string) error: %v", err)
	}
	if scanned.Val.Name != "Alice" || scanned.Val.Age != 30 {
		t.Errorf("Scan mismatch: got %+v", scanned.Val)
	}

	// Scan from []byte
	var scanned2 r3.JSONColumn[Inner]
	if err := scanned2.Scan([]byte(str)); err != nil {
		t.Fatalf("Scan([]byte) error: %v", err)
	}
	if scanned2.Val.Name != "Alice" || scanned2.Val.Age != 30 {
		t.Errorf("Scan []byte mismatch: got %+v", scanned2.Val)
	}
}

func TestJSONColumn_ScanNil(t *testing.T) {
	col := r3.NewJSONColumn([]string{"a", "b"})

	if err := col.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if col.Val != nil {
		t.Errorf("expected nil after Scan(nil), got %v", col.Val)
	}
}

func TestJSONColumn_ScanUnsupportedType(t *testing.T) {
	var col r3.JSONColumn[string]
	if err := col.Scan(42); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestJSONColumn_JSONMarshalUnmarshal(t *testing.T) {
	type Wrapper struct {
		Items r3.JSONColumn[[]string] `json:"items"`
	}

	original := Wrapper{Items: r3.NewJSONColumn([]string{"x", "y", "z"})}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// JSON output should be transparent — items is a plain array, not a nested object
	expected := `{"items":["x","y","z"]}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}

	var restored Wrapper
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if len(restored.Items.Val) != 3 || restored.Items.Val[0] != "x" {
		t.Errorf("Unmarshal mismatch: got %+v", restored.Items.Val)
	}
}

func TestJSONColumn_SliceType(t *testing.T) {
	type Change struct {
		Field string `json:"field"`
		Value int    `json:"value"`
	}

	col := r3.NewJSONColumn([]Change{
		{Field: "total", Value: 100},
		{Field: "count", Value: 5},
	})

	val, err := col.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}

	var restored r3.JSONColumn[[]Change]
	if err := restored.Scan(val); err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	if len(restored.Val) != 2 {
		t.Fatalf("expected 2 items, got %d", len(restored.Val))
	}
	if restored.Val[0].Field != "total" || restored.Val[0].Value != 100 {
		t.Errorf("item 0 mismatch: %+v", restored.Val[0])
	}
}

func TestJSONColumn_MapType(t *testing.T) {
	col := r3.NewJSONColumn(map[string]string{
		"actor_id": "user_42",
		"source":   "api",
	})

	val, err := col.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}

	var restored r3.JSONColumn[map[string]string]
	if err := restored.Scan(val); err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	if restored.Val["actor_id"] != "user_42" {
		t.Errorf("expected actor_id=user_42, got %q", restored.Val["actor_id"])
	}
}
