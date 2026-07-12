package r3

// RelationKind identifies the shape of a relation between two entities.
type RelationKind uint8

const (
	// RelationHasMany is one-to-many: the child rows carry an FK to the owner.
	RelationHasMany RelationKind = iota + 1
	// RelationBelongsTo is many-to-one: the owner row carries an FK to the related row.
	RelationBelongsTo
	// RelationManyToMany is resolved through a join table.
	RelationManyToMany
)

// String returns the canonical lowercase name matching the `r3:"rel:..."` tag
// vocabulary ("has-many" | "belongs-to" | "many-to-many").
func (k RelationKind) String() string {
	switch k {
	case RelationHasMany:
		return "has-many"
	case RelationBelongsTo:
		return "belongs-to"
	case RelationManyToMany:
		return "many-to-many"
	default:
		return "unknown"
	}
}

// RelationSpec physically describes a relation by table and column names, so a
// driver can resolve it WITHOUT importing the related Go type - the explicit
// counterpart to a tag-declared relation (`r3:"rel:..."`). This lets an entity
// relate to a table it cannot import (e.g. when the packages already reference
// each other, so importing would create a domain cycle); being a value rather
// than a struct field, it needs no Go field on the entity.
//
// Register specs with [WithRelations]; they then resolve by name through
// [Has]/[HasNo] and [AggregateThroughRelation] like tag-declared relations, but
// are not preloadable (no struct field to load into). Build one with
// [HasManyRelation], [BelongsToRelation], or [ManyToManyRelation].
type RelationSpec struct {
	// Name is the logical name used by Has/HasNo and AggregateThroughRelation;
	// it need not correspond to any struct field.
	Name string

	// Kind is the relation shape.
	Kind RelationKind

	// TargetTable is the related rows' table (e.g. "locations").
	TargetTable string

	// TargetPK is the related table's primary-key column. Defaults to "id".
	TargetPK string

	// FKColumn is the foreign-key column:
	//   - has-many:     the FK on the related (child) table pointing at the owner
	//   - belongs-to:   the FK on the owner table pointing at the related row
	//   - many-to-many: the owner-side FK column in the join table
	FKColumn string

	// RefColumn is the related-side FK column in the join table (many-to-many only).
	RefColumn string

	// JoinTable is the join table name (many-to-many only).
	JoinTable string

	// TargetSoftDeleteColumn, when set, is the related table's soft-delete
	// column: [AggregateThroughRelation] excludes related rows whose value is
	// non-NULL, so soft-deleted related rows are not counted.
	TargetSoftDeleteColumn string
}

// RelationOption customizes an optional field of a [RelationSpec].
type RelationOption func(*RelationSpec)

// RelationTargetPK overrides the related table's primary-key column (default "id").
func RelationTargetPK(column string) RelationOption {
	return func(s *RelationSpec) { s.TargetPK = column }
}

// RelationTargetSoftDelete declares the related table's soft-delete column so
// [AggregateThroughRelation] excludes soft-deleted related rows.
func RelationTargetSoftDelete(column string) RelationOption {
	return func(s *RelationSpec) { s.TargetSoftDeleteColumn = column }
}

// HasManyRelation declares a one-to-many relation: `fkColumn` on `targetTable`
// points back at the owner's primary key.
func HasManyRelation(name, targetTable, fkColumn string, opts ...RelationOption) RelationSpec {
	return newRelationSpec(RelationHasMany, name, targetTable, fkColumn, "", "", opts)
}

// BelongsToRelation declares a many-to-one relation: the owner's `fkColumn`
// points at `targetTable`'s primary key.
func BelongsToRelation(name, targetTable, fkColumn string, opts ...RelationOption) RelationSpec {
	return newRelationSpec(RelationBelongsTo, name, targetTable, fkColumn, "", "", opts)
}

// ManyToManyRelation declares a many-to-many relation through `joinTable`:
// `fkColumn` is the owner-side FK and `refColumn` the related-side FK.
func ManyToManyRelation(
	name, joinTable, fkColumn, refColumn, targetTable string, opts ...RelationOption,
) RelationSpec {
	return newRelationSpec(RelationManyToMany, name, targetTable, fkColumn, refColumn, joinTable, opts)
}

func newRelationSpec(
	kind RelationKind, name, targetTable, fkColumn, refColumn, joinTable string, opts []RelationOption,
) RelationSpec {
	s := RelationSpec{
		Name:        name,
		Kind:        kind,
		TargetTable: targetTable,
		TargetPK:    "id",
		FKColumn:    fkColumn,
		RefColumn:   refColumn,
		JoinTable:   joinTable,
	}
	for _, opt := range opts {
		opt(&s)
	}
	return s
}
