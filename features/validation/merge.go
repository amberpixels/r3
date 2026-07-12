package validation

import (
	"reflect"

	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
)

// mergePatch overlays the patched fields onto a copy of existing, so validators
// see the full post-patch entity rather than a sparse input whose non-patched
// fields are zeroed (which naive "required" rules would wrongly reject).
//
// Field names resolve to struct fields via the engines' column-tag conventions
// (r3 -> db -> snake_case); unresolved names are ignored (the backend rejects
// unknown columns). A non-struct T (pointer, map) can't be merged, so patch is
// returned unchanged.
func mergePatch[T any](existing, patch T, fields r3.Fields) T {
	et := reflect.TypeOf(existing)
	if et == nil || et.Kind() != reflect.Struct {
		return patch
	}

	merged := existing // value copy of the struct
	mv := reflect.ValueOf(&merged).Elem()
	pv := reflect.ValueOf(patch)

	for _, f := range fields {
		if f == nil {
			continue
		}
		idx := fieldIndexForColumn(et, f.String())
		if idx < 0 {
			continue
		}
		mv.Field(idx).Set(pv.Field(idx))
	}
	return merged
}

// fieldIndexForColumn returns the struct field index whose resolved column name
// equals column, or -1 if none matches.
func fieldIndexForColumn(t reflect.Type, column string) int {
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		ct := r3tag.ParseColumnTag(f)
		if ct.Skip {
			continue
		}
		if ct.Column == column {
			return i
		}
	}
	return -1
}
