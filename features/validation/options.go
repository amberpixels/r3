package validation

import (
	"reflect"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// IDFunc extracts the primary key from an entity, letting the decorator fetch
// the existing entity for state-transition validation (Update, Patch).
type IDFunc[T any, ID comparable] func(entity T) ID

// Options configures the validation decorator.
type Options[T any, ID comparable] struct {
	// IDFunc, when set, makes the decorator fetch the existing entity before
	// Update and Patch, populating Request.Existing for state-transition
	// validation. Without it, Existing is always nil.
	IDFunc IDFunc[T, ID]

	// RecordType names the entity type in error messages. If empty, derived from
	// T (e.g. Order -> "orders", OrderItem -> "order_items").
	RecordType string
}

// Option is a functional option for configuring the validation CRUD decorator.
type Option[T any, ID comparable] func(*Options[T, ID])

// WithIDFunc sets the primary-key extractor, enabling state-transition validation:
// the decorator fetches current DB state before Update and Patch so the validator
// can compare old vs new.
func WithIDFunc[T any, ID comparable](fn IDFunc[T, ID]) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.IDFunc = fn
	}
}

// WithRecordType sets an explicit entity type name for error messages.
func WithRecordType[T any, ID comparable](name string) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.RecordType = name
	}
}

// applyDefaults fills in zero-value options with sensible defaults.
func applyDefaults[T any, ID comparable](opts *Options[T, ID]) {
	if opts.RecordType == "" {
		opts.RecordType = deriveRecordType[T]()
	}
}

// typeName returns the unqualified struct name of T (e.g. "Order", "OrderItem").
func typeName[T any]() string {
	var zero T
	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

// deriveRecordType derives the record type name from the struct type T.
// It converts the struct name to snake_case plural (e.g. Order -> "orders").
func deriveRecordType[T any]() string {
	return r3utils.ToSnakeCasePlural(typeName[T]())
}
