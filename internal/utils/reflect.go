package r3utils

import (
	"reflect"
	"time"
)

// timeType is used to distinguish time.Time from other structs during reflection.
var timeType = reflect.TypeFor[time.Time]()

// IsRelationType returns true if the Go type represents a relation field
// (slice, map, pointer-to-struct, or struct — except time.Time).
func IsRelationType(t reflect.Type) bool {
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
