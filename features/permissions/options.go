package permissions

import (
	"reflect"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// IDFunc extracts the primary key from an entity.
// Used by the decorator to fetch existing entities for entity-aware checks
// (Update, Patch, Delete).
type IDFunc[T any, ID comparable] func(entity T) ID

// Options configures the behavior of the permissions decorator.
type Options[T any, ID comparable] struct {
	// IDFunc extracts the primary key from an entity.
	// When set, the decorator fetches the existing entity before Update, Patch,
	// and Delete operations, enabling entity-aware (row-level) permission checks.
	// Without IDFunc, only resource-level checks are performed for mutations.
	IDFunc IDFunc[T, ID]

	// RecordType is the entity type name used in error messages.
	// If empty, it is derived automatically from the struct type T
	// (e.g. Order -> "orders", CampaignAdset -> "campaign_adsets").
	RecordType string
}

// Option is a functional option for configuring the permissions CRUD decorator.
type Option[T any, ID comparable] func(*Options[T, ID])

// WithIDFunc sets the function that extracts the primary key from an entity.
// This enables entity-aware (row-level) permission checks for Update, Patch,
// and Delete operations.
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

// typeName returns the unqualified struct name of T (e.g. "Order", "CampaignAdset").
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
