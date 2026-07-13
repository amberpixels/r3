package enginemongo

import (
	"github.com/amberpixels/r3"
)

// RelationMetaFromSpec converts a core [r3.RelationSpec] (a relation declared by
// collection and field names) into a [RelationMeta]. The target Go type is unknown
// - the point of an explicit spec - so TargetType is nil and FieldIndex is -1: the
// relation is filterable and aggregatable but has no struct field to preload into.
func RelationMetaFromSpec(spec r3.RelationSpec) RelationMeta {
	// The core spec defaults TargetPK to "id" (the SQL convention); Mongo stores the
	// primary key as "_id", so map the default (and an explicit "id") accordingly.
	pk := spec.TargetPK
	if pk == "" || pk == "id" {
		pk = bsonIDField
	}
	return RelationMeta{
		FieldName:  spec.Name,
		FieldIndex: -1,
		Kind:       relationKindFromSpec(spec.Kind),
		FKField:    spec.FKColumn,
		RefField:   spec.RefColumn,
		JoinTable:  spec.JoinTable,
		TargetMeta: StructMeta{
			CollectionName:  spec.TargetTable,
			IDField:         pk,
			SoftDeleteField: spec.TargetSoftDeleteColumn,
		},
	}
}

// relationKindFromSpec maps a core relation kind to the engine's kind enum.
func relationKindFromSpec(k r3.RelationKind) RelationKind {
	switch k {
	case r3.RelationHasMany:
		return RelHasMany
	case r3.RelationBelongsTo:
		return RelBelongsTo
	case r3.RelationManyToMany:
		return RelManyToMany
	default:
		return 0
	}
}

// WithDeclaredRelations returns a copy of meta with specs added to its Relations.
// A declared relation whose name matches a tag-reflected one replaces it (explicit
// spec wins). The input meta is never mutated (shared and cached); nil/empty specs
// return meta unchanged.
func WithDeclaredRelations(meta StructMeta, specs []r3.RelationSpec) StructMeta {
	if len(specs) == 0 {
		return meta
	}

	relations := make([]RelationMeta, 0, len(meta.Relations)+len(specs))
	relations = append(relations, meta.Relations...)

	byName := make(map[string]int, len(relations))
	for i, rel := range relations {
		byName[rel.FieldName] = i
	}
	for _, spec := range specs {
		rm := RelationMetaFromSpec(spec)
		if i, ok := byName[rm.FieldName]; ok {
			relations[i] = rm
			continue
		}
		byName[rm.FieldName] = len(relations)
		relations = append(relations, rm)
	}

	meta.Relations = relations
	return meta
}
