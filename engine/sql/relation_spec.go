package enginesql

import (
	"github.com/amberpixels/r3"
	r3tag "github.com/amberpixels/r3/internal/tag"
)

// RelationMetaFromSpec converts a core [r3.RelationSpec] (a relation declared by
// table and column names) into the driver-facing [RelationMeta]. The target Go
// type is unknown — that is the point of an explicit spec — so TargetType is nil
// and FieldIndex is -1: a declared relation is filterable and aggregatable but
// has no struct field to preload into.
func RelationMetaFromSpec(spec r3.RelationSpec) RelationMeta {
	pk := spec.TargetPK
	if pk == "" {
		pk = "id"
	}
	return RelationMeta{
		FieldName:  spec.Name,
		FieldIndex: -1,
		Kind:       relationKindFromSpec(spec.Kind),
		FKColumn:   spec.FKColumn,
		RefColumn:  spec.RefColumn,
		JoinTable:  spec.JoinTable,
		TargetMeta: StructMeta{
			TableName:        spec.TargetTable,
			PKColumn:         pk,
			SoftDeleteColumn: spec.TargetSoftDeleteColumn,
		},
	}
}

func relationKindFromSpec(k r3.RelationKind) RelationKind {
	switch k {
	case r3.RelationHasMany:
		return r3tag.RelHasMany
	case r3.RelationBelongsTo:
		return r3tag.RelBelongsTo
	case r3.RelationManyToMany:
		return r3tag.RelManyToMany
	default:
		return 0
	}
}

// WithDeclaredRelations returns a copy of meta whose Relations include the given
// specs. A declared relation whose name matches an existing (tag-reflected)
// relation replaces it — an explicit spec overrides a tag-derived one. The input
// meta is never mutated (it is shared and cached), and a nil/empty specs list
// returns meta unchanged.
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
