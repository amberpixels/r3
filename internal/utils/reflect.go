package r3utils

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"time"
)

// timeType is used to distinguish time.Time from other structs during reflection.
var timeType = reflect.TypeFor[time.Time]()

// valuerType and scannerType detect types that persist as a single SQL column
// through database/sql (e.g. r3.JSONColumn) rather than as a relation.
var (
	valuerType  = reflect.TypeFor[driver.Valuer]()
	scannerType = reflect.TypeFor[sql.Scanner]()
)

// IsRelationType returns true if the Go type represents a relation field
// (slice, map, pointer-to-struct, or struct — except time.Time and types that
// round-trip as a single column via database/sql).
//
// A struct/slice/map that implements driver.Valuer or sql.Scanner (checked on
// the type and its pointer, since Scan's receiver is conventionally a pointer)
// is a scalar-like column, not a relation — this is what makes r3.JSONColumn
// persist as a JSON column instead of being silently dropped.
func IsRelationType(t reflect.Type) bool {
	if isColumnValuer(t) {
		return false
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Map:
		return true
	case reflect.Pointer:
		return t.Elem().Kind() == reflect.Struct && t.Elem() != timeType
	case reflect.Struct:
		return t != timeType
	default:
		return false
	}
}

// isColumnValuer reports whether t (or *t) implements driver.Valuer or
// sql.Scanner, meaning it maps to one SQL column.
func isColumnValuer(t reflect.Type) bool {
	if t == nil {
		return false
	}
	pt := reflect.PointerTo(t)
	return t.Implements(valuerType) || t.Implements(scannerType) ||
		pt.Implements(valuerType) || pt.Implements(scannerType)
}

// ResolveElementType unwraps slice and pointer types to get the underlying
// struct type. E.g. []CityTranslation -> CityTranslation, *City -> City.
func ResolveElementType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Slice:
		elem := t.Elem()
		if elem.Kind() == reflect.Pointer {
			return elem.Elem()
		}
		return elem
	case reflect.Pointer:
		return t.Elem()
	default:
		return t
	}
}
