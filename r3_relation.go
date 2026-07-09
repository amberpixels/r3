package r3

// RelationKind identifies the shape of a relation between two entities.
type RelationKind uint8

const (
	// RelationHasMany is a one-to-many relation: the related (child) rows carry
	// a foreign key back to the owner (e.g. City has many Translations).
	RelationHasMany RelationKind = iota + 1
	// RelationBelongsTo is a many-to-one relation: the owner row carries a
	// foreign key to the related row (e.g. Location belongs to City).
	RelationBelongsTo
	// RelationManyToMany is a many-to-many relation resolved through a join
	// table (e.g. Event has many Artists via artist_to_events).
	RelationManyToMany
)

// String returns the canonical lowercase name of the relation kind
// ("has-many" | "belongs-to" | "many-to-many"), matching the `r3:"rel:..."`
// tag vocabulary.
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

// RelationSpec physically describes a relation so a driver can resolve it
// WITHOUT importing the related Go type. It is the explicit counterpart to a
// tag-declared relation (`r3:"rel:..."`): where a tagged relation reflects the
// target table and primary key from the related struct, a RelationSpec states
// them directly as table and column names.
//
// This lets an entity declare a relation to a table it cannot (or should not)
// import as a Go type — for example when importing the related package would
// create a domain import cycle (the queried entity and the relation target
// already reference each other). Because a RelationSpec is not a struct field,
// it needs no Go field on the entity at all.
//
// Register specs on a repository with [WithRelations]. They then resolve by
// name through [Has]/[HasNo] filters and [AggregateThroughRelation], exactly
// like tag-declared relations. A declared relation is filterable and
// aggregatable but not preloadable (there is no struct field to load into).
//
// Build one with [HasManyRelation], [BelongsToRelation], or
// [ManyToManyRelation] rather than assembling the struct by hand.
type RelationSpec struct {
	// Name is the logical relation name used by Has("name")/HasNo("name") and
	// AggregateThroughRelation. It need not correspond to any struct field.
	Name string

	// Kind is the relation shape (has-many, belongs-to, many-to-many).
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

	// RefColumn is the related-side FK column in the join table.
	// Many-to-many only.
	RefColumn string

	// JoinTable is the join table name. Many-to-many only.
	JoinTable string

	// TargetSoftDeleteColumn, when set, is the related table's soft-delete
	// column. [AggregateThroughRelation] excludes related rows whose value is
	// non-NULL, so soft-deleted related rows are not counted.
	TargetSoftDeleteColumn string
}

// RelationOption customizes an optional field of a [RelationSpec].
type RelationOption func(*RelationSpec)

// RelationTargetPK overrides the related table's primary-key column
// (default "id").
func RelationTargetPK(column string) RelationOption {
	return func(s *RelationSpec) { s.TargetPK = column }
}

// RelationTargetSoftDelete declares the related table's soft-delete column so
// [AggregateThroughRelation] excludes soft-deleted related rows.
func RelationTargetSoftDelete(column string) RelationOption {
	return func(s *RelationSpec) { s.TargetSoftDeleteColumn = column }
}

// HasManyRelation declares a one-to-many relation named `name` whose related
// rows live in `targetTable` and carry `fkColumn` pointing back at the owner's
// primary key.
func HasManyRelation(name, targetTable, fkColumn string, opts ...RelationOption) RelationSpec {
	return newRelationSpec(RelationHasMany, name, targetTable, fkColumn, "", "", opts)
}

// BelongsToRelation declares a many-to-one relation named `name`: the owner row
// carries `fkColumn` pointing at `targetTable`'s primary key.
func BelongsToRelation(name, targetTable, fkColumn string, opts ...RelationOption) RelationSpec {
	return newRelationSpec(RelationBelongsTo, name, targetTable, fkColumn, "", "", opts)
}

// ManyToManyRelation declares a many-to-many relation named `name` resolved
// through `joinTable`: `fkColumn` is the owner-side FK and `refColumn` the
// related-side FK in the join table, with related rows in `targetTable`.
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
