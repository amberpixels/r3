package r3

import (
	"reflect"
	"sync"
	"time"

	r3tag "github.com/amberpixels/r3/internal/tag"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// timeType lets derivation distinguish time.Time (a scalar) from other structs
// (which are relations or JSON blobs).
var timeType = reflect.TypeFor[time.Time]()

// schemaCache memoizes the default (no-option) Schema per reflect.Type. Schema
// derivation reflects over tags, so caching keeps it off the hot path the way
// the engine caches StructMeta.
var schemaCache sync.Map // reflect.Type -> Schema

// schemaConfig is the resolved derivation policy.
type schemaConfig struct {
	naming NamingConfig
}

// SchemaOption customizes schema derivation.
type SchemaOption func(*schemaConfig)

// WithSchemaNaming overrides the well-known field names (created_at, updated_at,
// deleted_at) used to derive the read-only timestamp/soft-delete defaults. By
// default the standard names apply.
func WithSchemaNaming(n NamingConfig) SchemaOption {
	return func(c *schemaConfig) { c.naming = n }
}

// SchemaOf reflects T's struct tags into a logical Schema with default
// capabilities (see the schema design doc, §2.7): a plain scalar column is
// queryable, filterable, sortable, creatable, and mutable; the PK, the
// created_at/updated_at timestamps, and the soft-delete column are read-only;
// relations are queryable (preload) only. Tags only ever tighten these defaults.
//
// The default (no-option) result is cached per type T.
func SchemaOf[T any](opts ...SchemaOption) Schema {
	typ := derefType(reflect.TypeFor[T]())
	if typ == nil || typ.Kind() != reflect.Struct {
		return Schema{}
	}

	if len(opts) == 0 {
		if cached, ok := schemaCache.Load(typ); ok {
			if s, ok := cached.(Schema); ok {
				return s
			}
		}
		s := deriveSchema(typ, resolveSchemaConfig(nil))
		schemaCache.Store(typ, s)
		return s
	}

	return deriveSchema(typ, resolveSchemaConfig(opts))
}

// resolveSchemaConfig applies options over the standard naming defaults.
func resolveSchemaConfig(opts []SchemaOption) schemaConfig {
	c := schemaConfig{naming: DefaultConfig().Naming}
	for _, opt := range opts {
		opt(&c)
	}
	// Backfill any names the caller left empty so detection is never blank.
	def := DefaultConfig().Naming
	if c.naming.CreatedAtField == "" {
		c.naming.CreatedAtField = def.CreatedAtField
	}
	if c.naming.UpdatedAtField == "" {
		c.naming.UpdatedAtField = def.UpdatedAtField
	}
	if c.naming.DeletedAtField == "" {
		c.naming.DeletedAtField = def.DeletedAtField
	}
	return c
}

// deriveSchema walks the struct fields once, mirroring engine/sql's field
// classification (relation vs column) so the logical schema stays 1:1 with the
// physical columns the engine binds.
func deriveSchema(typ reflect.Type, cfg schemaConfig) Schema {
	var attrs []Attribute
	for field := range typ.Fields() {
		if !field.IsExported() {
			continue
		}

		if r3utils.IsRelationType(field.Type) {
			if rel, ok := r3tag.ParseRelationTag(field); ok {
				attrs = append(attrs, relationAttribute(field, rel))
			}
			continue
		}

		ct := r3tag.ParseColumnTag(field)
		if ct.Skip {
			continue
		}
		attrs = append(attrs, columnAttribute(field, ct, cfg.naming))
	}
	return newSchema(attrs)
}

// columnAttribute derives a scalar/JSON attribute from a column field: it infers
// the data type, computes the default capabilities, then tightens them with any
// tag flags.
func columnAttribute(field reflect.StructField, ct r3tag.ColumnTag, naming NamingConfig) Attribute {
	dt := inferType(field.Type, ct)
	caps := defaultColumnCaps(ct, dt, naming)

	// Tag flags only tighten (clear) capabilities — they never widen.
	if ct.NoFilter {
		caps &^= Filterable
	}
	if ct.NoSort {
		caps &^= Sortable
	}
	if ct.NoOutput {
		caps &^= Queryable
	}
	if ct.ReadOnly {
		caps &^= Creatable | Mutable
	}
	if ct.Immutable {
		caps &^= Mutable
	}

	attr := Attribute{
		Name: ct.Column,
		Type: dt,
		Caps: caps,
	}
	if caps&Filterable != 0 {
		attr.Ops = defaultOps(dt)
	}
	if dt == TypeEnum {
		attr.Enum = ct.Enum
	}
	return attr
}

// defaultColumnCaps applies the permissive default policy: full capabilities for
// a plain scalar, minus the structural exceptions (PK, timestamps, soft-delete
// are read-only) and minus filter/sort for non-scalar types.
func defaultColumnCaps(ct r3tag.ColumnTag, dt DataType, naming NamingConfig) Capability {
	caps := capsAll
	if !dt.isScalar() {
		caps &^= Filterable | Sortable
	}

	// PK is identity, never written by a caller.
	if ct.IsPK || ct.Column == "id" {
		caps &^= Creatable | Mutable
	}
	// Timestamps are server-managed; updated_at is system-only.
	if ct.Column == naming.CreatedAtField || ct.Column == naming.UpdatedAtField {
		caps &^= Creatable | Mutable
	}
	// Soft-delete column is managed by Delete/Restore, not a caller write.
	if ct.SoftDelete || ct.Column == naming.DeletedAtField {
		caps &^= Creatable | Mutable
	}
	return caps
}

// relationAttribute derives a queryable-only relation attribute. Relations are
// preloadable but not filterable/sortable as plain fields (relationship filters
// use the dedicated Has mechanism).
func relationAttribute(field reflect.StructField, rel r3tag.RelationTag) Attribute {
	target := r3utils.ToSnakeCase(r3utils.ResolveElementType(field.Type).Name())
	return Attribute{
		Name: r3utils.ToSnakeCase(field.Name),
		Type: TypeRel,
		Caps: Queryable,
		Relation: &RelationRef{
			Target: target,
			Kind:   relationKindString(rel.Kind),
		},
	}
}

// inferType maps a Go field type to a logical DataType. An explicit enum tag
// wins; otherwise the Go kind decides. Only scalars and time.Time reach here as
// columns — other structs/slices/maps are classified as relations upstream — so
// the JSON branch is a defensive fallback.
func inferType(t reflect.Type, ct r3tag.ColumnTag) DataType {
	if len(ct.Enum) > 0 {
		return TypeEnum
	}
	t = derefType(t)
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypeInt
	case reflect.Float32, reflect.Float64:
		return TypeFloat
	case reflect.Bool:
		return TypeBool
	case reflect.String:
		return TypeString
	case reflect.Struct:
		if t == timeType {
			return TypeTime
		}
		return TypeJSON
	default:
		// Slices/maps/other shapes that slip past relation classification are
		// modeled as opaque JSON blobs.
		return TypeJSON
	}
}

// derefType unwraps a single pointer level (nullable column) to the element type.
func derefType(t reflect.Type) reflect.Type {
	if t != nil && t.Kind() == reflect.Pointer {
		return t.Elem()
	}
	return t
}

// relationKindString renders a relation kind as its stable wire string.
func relationKindString(k r3tag.RelationKind) string {
	switch k {
	case r3tag.RelHasMany:
		return "has-many"
	case r3tag.RelBelongsTo:
		return "belongs-to"
	case r3tag.RelManyToMany:
		return "many-to-many"
	default:
		return ""
	}
}
