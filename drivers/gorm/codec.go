package r3gorm

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"github.com/amberpixels/r3"
	"gorm.io/gorm"
	gschema "gorm.io/gorm/schema"
)

// This file bridges an r3 value codec (r3:"...,codec:<name>") to GORM's serializer
// mechanism so the user needs no gorm:"serializer:..." tag.
//
// GORM bakes a field's scan/value closures from its tag during schema.Parse
// (schema.Field.setupValuerAndSetter), so a serializer set after Parse is ignored,
// and GORM's serializer wrapper type is unexported (not reproducible from outside).
// The bridge therefore parses a shadow struct of identical layout, but with a
// generated gorm:"serializer:<name>" tag on the codec'd field, then grafts the
// resulting closures onto the real cached schema's field - keeping the real
// schema's own table name and model type.

// codecSerializer adapts an r3.Codec to gorm/schema.SerializerInterface. GORM
// clones it per use via reflection, preserving the codec reference.
type codecSerializer struct{ codec r3.Codec }

var _ gschema.SerializerInterface = codecSerializer{}

// Value encodes the Go field value to its stored form for binding.
func (s codecSerializer) Value(
	_ context.Context, _ *gschema.Field, _ reflect.Value, fieldValue any,
) (any, error) {
	return s.codec.Encode(fieldValue)
}

// Scan decodes a stored column value back into the Go field.
func (s codecSerializer) Scan(
	ctx context.Context, field *gschema.Field, dst reflect.Value, dbValue any,
) error {
	goVal, err := s.codec.Decode(dbValue, field.FieldType)
	if err != nil {
		return err
	}
	return field.Set(ctx, dst, goVal)
}

// codecSerializerNames memoizes the generated GORM serializer name per distinct
// r3.Codec instance, so the same codec registers exactly one GORM serializer.
var (
	codecSerializerMu    sync.Mutex
	codecSerializerNames = map[r3.Codec]string{}
	codecSerializerSeq   int
)

// registerCodecSerializer returns the GORM serializer name bound to c, registering
// a fresh codecSerializer under a generated name on first use. The name is prefixed
// to avoid colliding with GORM's built-ins (json, unixtime, gob).
func registerCodecSerializer(c r3.Codec) string {
	codecSerializerMu.Lock()
	defer codecSerializerMu.Unlock()
	// Memoize by codec identity, but only when the concrete type is comparable (a
	// map key must be); a rare non-comparable custom codec just registers a fresh
	// serializer each time rather than panicking on the map lookup.
	memoizable := reflect.TypeOf(c).Comparable()
	if memoizable {
		if name, ok := codecSerializerNames[c]; ok {
			return name
		}
	}
	codecSerializerSeq++
	name := "r3codec_" + strconv.Itoa(codecSerializerSeq)
	if memoizable {
		codecSerializerNames[c] = name
	}
	gschema.RegisterSerializer(name, codecSerializer{codec: c})
	return name
}

// wireCodecs makes GORM apply the schema's value codecs to T's fields: for each
// codec'd attribute it registers a serializer and grafts its scan/value closures
// (from a shadow parse) onto the real cached schema. No-op without codecs. A
// structural mismatch is a programming error and panics, like the unknown-codec /
// unsupported-backend guards.
func wireCodecs[T any](db *gorm.DB, schema r3.Schema) {
	codecs := codecAttributes(schema)
	if len(codecs) == 0 {
		return
	}

	// The real, cached schema GORM will reuse for T.
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(new(T)); err != nil {
		panic(fmt.Errorf("r3/gorm: parsing schema to wire codecs: %w", err))
	}
	realSchema := stmt.Schema

	// A shadow type of the same layout, but with a generated gorm serializer tag on
	// each codec'd field so its closures get built. The column comes from the real
	// parsed schema (keyed by Go field name), matching GORM's own column derivation.
	realType := reflect.TypeFor[T]()
	fields := make([]reflect.StructField, realType.NumField())
	wired := map[string]bool{} // columns we grafted (for the post-check)
	for i := range realType.NumField() {
		f := realType.Field(i)
		if rf, ok := realSchema.FieldsByName[f.Name]; ok {
			if c, isCodec := codecs[rf.DBName]; isCodec {
				name := registerCodecSerializer(c)
				f.Tag = reflect.StructTag(
					fmt.Sprintf(`gorm:"column:%s;serializer:%s"`, rf.DBName, name),
				)
				wired[rf.DBName] = true
			}
		}
		fields[i] = f
	}
	var shadowCache sync.Map
	shadowSchema, err := gschema.Parse(
		reflect.New(reflect.StructOf(fields)).Interface(),
		&shadowCache,
		db.NamingStrategy,
	)
	if err != nil {
		panic(fmt.Errorf("r3/gorm: parsing shadow schema to wire codecs: %w", err))
	}

	for column := range codecs {
		if !wired[column] {
			// A codec'd attribute with no matching GORM field is a modelling error.
			panic(fmt.Errorf("r3/gorm: cannot wire codec for column %q (no matching field)", column))
		}
		realField := realSchema.LookUpField(column)
		shadowField := shadowSchema.LookUpField(column)
		if realField == nil || shadowField == nil {
			panic(fmt.Errorf("r3/gorm: cannot wire codec for column %q (field lookup failed)", column))
		}
		realField.Serializer = shadowField.Serializer
		realField.ValueOf = shadowField.ValueOf
		realField.Set = shadowField.Set
		realField.NewValuePool = shadowField.NewValuePool
		realField.DataType = shadowField.DataType
	}
}

// codecAttributes maps stored column name -> codec for every codec'd attribute in
// the schema (the attribute Name is the column).
func codecAttributes(schema r3.Schema) map[string]r3.Codec {
	out := map[string]r3.Codec{}
	for _, attr := range schema.Attributes() {
		if attr.Codec != nil {
			out[attr.Name] = attr.Codec
		}
	}
	return out
}
