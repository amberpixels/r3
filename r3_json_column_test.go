package r3_test

import (
	"encoding/json"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/expectto/be"
	"github.com/expectto/be/be_json"
	"github.com/expectto/be/be_reflected"
	betestify "github.com/expectto/be/x/testify"
)

func TestJSONColumn_ValueAndScan(t *testing.T) {
	type Inner struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	original := r3.NewJSONColumn(Inner{Name: "Alice", Age: 30})

	// Value() yields a JSON string; assert its shape with be_json.
	val, err := original.Value()
	betestify.Require(t, err, be.Succeed())
	betestify.Require(t, val, be_reflected.AsString())
	betestify.Assert(t, val, be.JSON(be_json.JsonAsString,
		be_json.HaveKeyValue("name", "Alice"),
		be_json.HaveKeyValue("age", be_reflected.AsFloat()),
	))

	str := val.(string)

	// Scan from string
	var scanned r3.JSONColumn[Inner]
	betestify.Require(t, scanned.Scan(str), be.Succeed())
	betestify.Assert(t, scanned.Val, be.Eq(Inner{Name: "Alice", Age: 30}))

	// Scan from []byte
	var scanned2 r3.JSONColumn[Inner]
	betestify.Require(t, scanned2.Scan([]byte(str)), be.Succeed())
	betestify.Assert(t, scanned2.Val, be.Eq(Inner{Name: "Alice", Age: 30}))
}

func TestJSONColumn_ScanNil(t *testing.T) {
	col := r3.NewJSONColumn([]string{"a", "b"})

	betestify.Require(t, col.Scan(nil), be.Succeed())
	betestify.Assert(t, col.Val, be.Nil())
}

func TestJSONColumn_ScanUnsupportedType(t *testing.T) {
	var col r3.JSONColumn[string]
	betestify.Assert(t, col.Scan(42), be.HaveOccurred())
}

func TestJSONColumn_JSONMarshalUnmarshal(t *testing.T) {
	type Wrapper struct {
		Items r3.JSONColumn[[]string] `json:"items"`
	}

	original := Wrapper{Items: r3.NewJSONColumn([]string{"x", "y", "z"})}

	data, err := json.Marshal(original)
	betestify.Require(t, err, be.Succeed())

	// JSON output is transparent: "items" is a plain array, not a nested object.
	betestify.Assert(t, string(data), be.JSON(be_json.JsonAsString,
		be_json.HaveKeyValue("items", be.Eq([]any{"x", "y", "z"})),
	))

	var restored Wrapper
	betestify.Require(t, json.Unmarshal(data, &restored), be.Succeed())
	betestify.Assert(t, restored.Items.Val, be.Eq([]string{"x", "y", "z"}))
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
	betestify.Require(t, err, be.Succeed())

	var restored r3.JSONColumn[[]Change]
	betestify.Require(t, restored.Scan(val), be.Succeed())
	betestify.Assert(t, restored.Val, be.HaveLength(2))
	betestify.Assert(t, restored.Val[0], be.Eq(Change{Field: "total", Value: 100}))
}

func TestJSONColumn_MapType(t *testing.T) {
	col := r3.NewJSONColumn(map[string]string{
		"actor_id": "user_42",
		"source":   "api",
	})

	val, err := col.Value()
	betestify.Require(t, err, be.Succeed())

	var restored r3.JSONColumn[map[string]string]
	betestify.Require(t, restored.Scan(val), be.Succeed())
	betestify.Assert(t, restored.Val, be.HaveKeyWithValue("actor_id", "user_42"))
}
