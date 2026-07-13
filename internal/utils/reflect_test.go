package r3utils_test

import (
	"database/sql/driver"
	"reflect"
	"testing"
	"time"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// valuerColumn implements driver.Valuer, standing in for r3.JSONColumn: a struct
// that must round-trip as a single SQL column, not a relation.
type valuerColumn struct{ v string }

func (c valuerColumn) Value() (driver.Value, error) { return c.v, nil }

type relStruct struct{ Name string }

func TestIsRelationType(t *testing.T) {
	cases := []struct {
		name string
		typ  reflect.Type
		want bool
	}{
		{"string", reflect.TypeFor[string](), false},
		{"int", reflect.TypeFor[int64](), false},
		{"time.Time is a column", reflect.TypeFor[time.Time](), false},
		{"pointer to time is a column", reflect.TypeFor[*time.Time](), false},
		{"plain struct is a relation", reflect.TypeFor[relStruct](), true},
		{"pointer to struct is a relation", reflect.TypeFor[*relStruct](), true},
		{"slice is a relation", reflect.TypeFor[[]relStruct](), true},
		{"map is a relation", reflect.TypeFor[map[string]int](), true},
		// Regression: a driver.Valuer struct (e.g. r3.JSONColumn) is a column,
		// not a relation — otherwise the SQL engine silently drops it and its
		// JSON/JSONB value is never written or read.
		{"driver.Valuer struct is a column", reflect.TypeFor[valuerColumn](), false},
		{"pointer to driver.Valuer struct is a column", reflect.TypeFor[*valuerColumn](), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := r3utils.IsRelationType(tc.typ); got != tc.want {
				t.Errorf("IsRelationType(%s) = %v, want %v", tc.typ, got, tc.want)
			}
		})
	}
}
