package enginemongo

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// RelationKind describes the type of relationship between two entities.
type RelationKind = r3tag.RelationKind

const (
	// RelHasMany represents a one-to-many relationship.
	RelHasMany = r3tag.RelHasMany
	// RelBelongsTo represents a many-to-one relationship.
	RelBelongsTo = r3tag.RelBelongsTo
	// RelManyToMany represents a many-to-many relationship via a join collection.
	RelManyToMany = r3tag.RelManyToMany

	// bsonIDField is the standard MongoDB primary key field name.
	bsonIDField = "_id"
)

// RelationMeta holds metadata about a relation - either reflected from a struct
// field (preloadable) or declared explicitly via [r3.WithRelations] (FieldIndex
// -1, TargetType nil).
type RelationMeta struct {
	FieldName  string       // relation name (Go field name, or spec name)
	FieldIndex int          // struct field index for preload assignment; -1 for a declared spec
	Kind       RelationKind // has-many, belongs-to, or many-to-many
	FKField    string       // foreign key field: child-side (has-many), owner-side (belongs-to), owner-side in join (m2m)
	RefField   string       // related-side FK field in the join collection (many-to-many only)
	JoinTable  string       // join collection name (many-to-many only)
	TargetMeta StructMeta   // metadata for the related entity (collection, IDField, SoftDeleteField)
	TargetType reflect.Type // reflect.Type of the target entity (element type); nil for a declared spec
}

// StructMeta is reflection-based metadata about a struct type T, the MongoDB
// equivalent of enginesql.StructMeta.
type StructMeta struct {
	CollectionName string   // e.g. "cities"
	Fields         []string // BSON field names in order, e.g. ["_id", "name", ...]
	FieldIndices   []int    // struct field index per BSON field
	IDField        string   // primary key BSON field name (defaults to "_id")
	IDFieldIdx     int      // index into Fields/FieldIndices for the ID entry

	// SoftDeleteField is the soft-delete BSON field name; empty disables soft-delete.
	SoftDeleteField string

	// SoftDeleteZero is the zero value of a non-pointer soft-delete field, which a
	// live record persists (a time.Time stores the zero BSON Date, not null), so
	// the "not deleted" filter must match it as well as null. Nil for a pointer
	// field (live records store null) or when soft-delete is disabled.
	SoftDeleteZero any

	// Codecs maps a BSON field name to the value codec declared on it
	// (r3:"...,codec:name"). Keyed by the physical bson name - not the schema
	// name - because reads/writes and filter args all reference the stored field.
	// Empty when the type declares no codecs.
	Codecs map[string]r3.Codec

	Relations []RelationMeta
}

// GetStructMeta derives collection name and field info from T. Field-name
// priority: `r3` tag, then `bson`, then `db`, then snake_case; a "-" in any tag
// skips the field.
func GetStructMeta[T any]() StructMeta {
	var t T
	typ := reflect.TypeOf(t)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ, true)
}

// getStructMetaForType derives StructMeta from a type, skipping relation parsing.
func getStructMetaForType(typ reflect.Type) StructMeta {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return buildStructMeta(typ, false)
}

// buildStructMeta is the shared implementation for struct metadata extraction.
func buildStructMeta(typ reflect.Type, parseRelations bool) StructMeta {
	meta := StructMeta{
		CollectionName: r3utils.ToSnakeCasePlural(typ.Name()),
		IDField:        bsonIDField,
		IDFieldIdx:     -1,
	}

	for i := range typ.NumField() {
		field := typ.Field(i)

		if !field.IsExported() {
			continue
		}

		// Relation fields are detected by Go type.
		if r3utils.IsRelationType(field.Type) {
			if parseRelations {
				if rel, ok := buildRelationMeta(field, i); ok {
					meta.Relations = append(meta.Relations, rel)
				}
			}
			continue
		}

		bsonName, isPK, isSoftDelete, skip := parseMongoFieldTag(field)
		if skip {
			continue
		}

		meta.Fields = append(meta.Fields, bsonName)
		meta.FieldIndices = append(meta.FieldIndices, i)

		// Resolve a value codec declared on the field, keyed by the bson name so the
		// read/write and filter paths (which reference the stored field) find it. An
		// unregistered codec name is a deterministic developer error (a typo), so fail
		// loudly - matching SchemaOf - rather than silently storing un-encoded values.
		if ct := r3tag.ParseColumnTag(field); ct.Codec != "" {
			c, ok := r3.LookupCodec(ct.Codec)
			if !ok {
				panic(fmt.Errorf("%w: %q on field %q", r3.ErrUnknownCodec, ct.Codec, field.Name))
			}
			if meta.Codecs == nil {
				meta.Codecs = make(map[string]r3.Codec)
			}
			meta.Codecs[bsonName] = c
		}

		if isPK || bsonName == bsonIDField {
			meta.IDField = bsonName
			meta.IDFieldIdx = len(meta.FieldIndices) - 1
		}

		if isSoftDelete {
			meta.SoftDeleteField = bsonName
			// Capture a non-pointer field's zero value: live records store it,
			// not null, so the "not deleted" filter must match it. See SoftDeleteZero.
			if field.Type.Kind() != reflect.Pointer {
				meta.SoftDeleteZero = reflect.Zero(field.Type).Interface()
			}
		}
	}

	return meta
}

// parseMongoFieldTag resolves a field's (bsonName, isPK, isSoftDelete, skip) from
// its tags. The `r3` tag wins, with the `bson` tag name used when r3 sets only
// flags (pk, soft_delete) but no explicit name.
func parseMongoFieldTag(field reflect.StructField) (string, bool, bool, bool) {
	r3Raw := field.Tag.Get("r3")
	bsonRaw := field.Tag.Get("bson")
	dbRaw := field.Tag.Get("db")

	if r3Raw == "-" || bsonRaw == "-" || dbRaw == "-" {
		return "", false, false, true
	}

	// Relation fields are handled elsewhere.
	if strings.HasPrefix(r3Raw, "rel:") || strings.Contains(r3Raw, ",rel:") {
		return "", false, false, true
	}

	tag := r3tag.ParseColumnTag(field)
	if tag.Skip {
		return "", false, false, true
	}

	bsonName := tag.Column
	isPK := tag.IsPK
	isSoftDelete := tag.SoftDelete

	if bsonRaw != "" && bsonRaw != "-" {
		bsonParts := strings.Split(bsonRaw, ",")
		bsonTagName := strings.TrimSpace(bsonParts[0])
		if bsonTagName != "" {
			if r3Raw == "" || isR3OnlyFlags(r3Raw) {
				bsonName = bsonTagName
			}
		}
	}

	// MongoDB convention: the PK "id" is stored as "_id".
	if bsonName == "id" && isPK {
		bsonName = bsonIDField
	}

	return bsonName, isPK, isSoftDelete, false
}

// isR3OnlyFlags reports whether the r3 tag holds only known flags, no column name.
func isR3OnlyFlags(raw string) bool {
	for p := range strings.SplitSeq(raw, ",") {
		p = strings.TrimSpace(p)
		switch p {
		case "pk", "soft_delete":
			continue
		default:
			if strings.HasPrefix(p, "rel:") || strings.HasPrefix(p, "fk:") || strings.HasPrefix(p, "table:") {
				continue
			}
			return false
		}
	}
	return true
}

// buildRelationMeta parses the r3 tag on a relation field.
func buildRelationMeta(field reflect.StructField, fieldIndex int) (RelationMeta, bool) {
	tag, ok := r3tag.ParseRelationTag(field)
	if !ok {
		return RelationMeta{}, false
	}

	targetType := r3utils.ResolveElementType(field.Type)
	targetMeta := getStructMetaForType(targetType)
	if tag.TableName != "" {
		targetMeta.CollectionName = tag.TableName
	}

	return RelationMeta{
		FieldName:  field.Name,
		FieldIndex: fieldIndex,
		Kind:       tag.Kind,
		FKField:    tag.FKColumn,
		TargetMeta: targetMeta,
		TargetType: targetType,
	}, true
}

// --------------------------------------------------------------------------
// StructMeta methods
// --------------------------------------------------------------------------

// NonIDFields returns all BSON field names except the ID field.
func (m *StructMeta) NonIDFields() []string {
	var fields []string
	for _, f := range m.Fields {
		if f != m.IDField {
			fields = append(fields, f)
		}
	}
	return fields
}

// FieldValues extracts the BSON field values from a struct in the same order as Fields.
func (m *StructMeta) FieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	vals := make([]any, len(m.FieldIndices))
	for i, idx := range m.FieldIndices {
		vals[i] = v.Field(idx).Interface()
	}
	return vals
}

// NonIDFieldValues extracts field values excluding the ID, for inserts.
func (m *StructMeta) NonIDFieldValues(entity any) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	var vals []any
	for i, idx := range m.FieldIndices {
		if m.Fields[i] != m.IDField {
			vals = append(vals, v.Field(idx).Interface())
		}
	}
	return vals
}

// IDValue extracts the ID field value from an entity.
func (m *StructMeta) IDValue(entity any) any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if m.IDFieldIdx >= 0 && m.IDFieldIdx < len(m.FieldIndices) {
		return v.Field(m.FieldIndices[m.IDFieldIdx]).Interface()
	}
	return nil
}

// SetIDValue sets the ID field value on an entity (via pointer).
func (m *StructMeta) SetIDValue(entityPtr any, val any) {
	v := reflect.ValueOf(entityPtr).Elem()
	if m.IDFieldIdx >= 0 && m.IDFieldIdx < len(m.FieldIndices) {
		idField := v.Field(m.FieldIndices[m.IDFieldIdx])
		idField.Set(reflect.ValueOf(val).Convert(idField.Type()))
	}
}

// FieldValuesForFields extracts field values from an entity for the given BSON field names.
func (m *StructMeta) FieldValuesForFields(entity any, fields []string) []any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	fieldToIdx := make(map[string]int, len(m.Fields))
	for i, f := range m.Fields {
		fieldToIdx[f] = m.FieldIndices[i]
	}

	vals := make([]any, 0, len(fields))
	for _, f := range fields {
		if idx, ok := fieldToIdx[f]; ok {
			vals = append(vals, v.Field(idx).Interface())
		}
	}
	return vals
}

// ValidatePatchFields checks that the given field names are valid for a Patch operation.
func (m *StructMeta) ValidatePatchFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return nil, r3.ErrNoPatchFields
	}

	known := make(map[string]bool, len(m.Fields))
	for _, f := range m.Fields {
		known[f] = true
	}

	for _, f := range fields {
		if !known[f] {
			return nil, fmt.Errorf("%w: %q does not exist", r3.ErrInvalidPatchField, f)
		}
		if f == m.IDField {
			return nil, fmt.Errorf("%w: %q is the ID field", r3.ErrInvalidPatchField, f)
		}
		if m.SoftDeleteField != "" && f == m.SoftDeleteField {
			return nil, fmt.Errorf("%w: %q is the soft-delete field", r3.ErrInvalidPatchField, f)
		}
	}

	return fields, nil
}

// ToBSONDoc converts an entity to a BSON document for insert/update, optionally
// omitting the ID.
func (m *StructMeta) ToBSONDoc(entity any, includeID bool) map[string]any {
	v := reflect.ValueOf(entity)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	doc := make(map[string]any, len(m.Fields))
	for i, idx := range m.FieldIndices {
		fieldName := m.Fields[i]
		if !includeID && fieldName == m.IDField {
			continue
		}
		doc[fieldName] = v.Field(idx).Interface()
	}
	return doc
}

// SetFieldsFromMap sets struct fields from a map of BSON field name -> value.
func (m *StructMeta) SetFieldsFromMap(entityPtr any, data map[string]any) {
	v := reflect.ValueOf(entityPtr).Elem()

	fieldToIdx := make(map[string]int, len(m.Fields))
	for i, f := range m.Fields {
		fieldToIdx[f] = m.FieldIndices[i]
	}

	for key, val := range data {
		if idx, ok := fieldToIdx[key]; ok {
			field := v.Field(idx)
			if val != nil {
				field.Set(reflect.ValueOf(val).Convert(field.Type()))
			}
		}
	}
}
