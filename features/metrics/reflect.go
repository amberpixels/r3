package metrics

import (
	"reflect"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// typeName returns the unqualified struct name of T (e.g. "Order", "CampaignAdset").
func typeName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// deriveRecordType is T's struct name as snake_case plural (e.g. Order -> "orders").
func deriveRecordType[T any]() string {
	return r3utils.ToSnakeCasePlural(typeName[T]())
}
