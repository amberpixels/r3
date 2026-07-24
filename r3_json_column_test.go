package r3_test

import (
	"encoding/json"
	"testing"

	"github.com/expectto/be"
	"github.com/expectto/be/be_json"
	"github.com/expectto/be/be_reflected"

	"github.com/amberpixels/r3"
)

func TestJSONColumn_ValueAndScan(t *testing.T) {
	type Inner struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	original := r3.NewJSONColumn(Inner{Name: "Alice", Age: 30})

	// Value() yields a JSON string; assert its shape with be_json.
	val, err := original.Value()
	be.NoError(t, err)
	be.RequireThat(t, val, be_reflected.AsString())
	be.AssertThat(t, val, be.JSON(be_json.JsonAsString,
		be_json.HaveKeyValue("name", "Alice"),
		be_json.HaveKeyValue("age", be_reflected.AsFloat()),
	))

	str := val.(string)

	// Scan from string
	var scanned r3.JSONColumn[Inner]
	be.NoError(t, scanned.Scan(str))
	be.AssertThat(t, scanned.Val, be.Eq(Inner{Name: "Alice", Age: 30}))

	// Scan from []byte
	var scanned2 r3.JSONColumn[Inner]
	be.NoError(t, scanned2.Scan([]byte(str)))
	be.AssertThat(t, scanned2.Val, be.Eq(Inner{Name: "Alice", Age: 30}))
}

func TestJSONColumn_ScanNil(t *testing.T) {
	col := r3.NewJSONColumn([]string{"a", "b"})

	be.NoError(t, col.Scan(nil))
	be.AssertThat(t, col.Val, be.Nil())
}

func TestJSONColumn_ScanUnsupportedType(t *testing.T) {
	var col r3.JSONColumn[string]
	be.AssertThat(t, col.Scan(42), be.HaveOccurred())
}

func TestJSONColumn_JSONMarshalUnmarshal(t *testing.T) {
	type Wrapper struct {
		Items r3.JSONColumn[[]string] `json:"items"`
	}

	original := Wrapper{Items: r3.NewJSONColumn([]string{"x", "y", "z"})}

	data, err := json.Marshal(original)
	be.NoError(t, err)

	// JSON output is transparent: "items" is a plain array, not a nested object.
	be.AssertThat(t, string(data), be.JSON(be_json.JsonAsString,
		be_json.HaveKeyValue("items", be.Eq([]any{"x", "y", "z"})),
	))

	var restored Wrapper
	be.NoError(t, json.Unmarshal(data, &restored))
	be.AssertThat(t, restored.Items.Val, be.Eq([]string{"x", "y", "z"}))
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
	be.NoError(t, err)

	var restored r3.JSONColumn[[]Change]
	be.NoError(t, restored.Scan(val))
	be.AssertThat(t, restored.Val, be.HaveLength(2))
	be.AssertThat(t, restored.Val[0], be.Eq(Change{Field: "total", Value: 100}))
}

func TestJSONColumn_MapType(t *testing.T) {
	col := r3.NewJSONColumn(map[string]string{
		"actor_id": "user_42",
		"source":   "api",
	})

	val, err := col.Value()
	be.NoError(t, err)

	var restored r3.JSONColumn[map[string]string]
	be.NoError(t, restored.Scan(val))
	be.AssertThat(t, restored.Val, be.HaveKeyWithValue("actor_id", "user_42"))
}
