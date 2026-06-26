package r3

// DataType is the logical type of an attribute. It is engine-agnostic — it
// drives the default filter operators and (later) a frontend's filter widgets,
// not any storage representation.
type DataType string

const (
	TypeInt    DataType = "int"
	TypeFloat  DataType = "float"
	TypeString DataType = "string"
	TypeBool   DataType = "bool"
	TypeTime   DataType = "time"
	TypeEnum   DataType = "enum"
	TypeJSON   DataType = "json"
	TypeRel    DataType = "relation"
)

// isScalar reports whether a DataType is a scalar that can be filtered and
// sorted by default. JSON blobs and relations are non-scalar: queryable, but
// not filterable or sortable without an explicit opt-in.
func (t DataType) isScalar() bool {
	switch t {
	case TypeInt, TypeFloat, TypeString, TypeBool, TypeTime, TypeEnum:
		return true
	case TypeJSON, TypeRel:
		return false
	default:
		return false
	}
}

// Capability is a bitset of what an Attribute is allowed to do. The five
// capabilities are the public contract — the ceiling of what any API caller may
// do. The permissions feature can only narrow this per-actor/row, never widen
// it (see the schema design doc, §2.3).
type Capability uint8

const (
	// Filterable means the attribute may appear in Query.Filters.
	Filterable Capability = 1 << iota
	// Sortable means the attribute may appear in Query.Sorts.
	Sortable
	// Queryable means the attribute may appear in Query.Fields (SELECT) and in serialized output.
	Queryable
	// Creatable means the attribute may be set by Create.
	Creatable
	// Mutable means the attribute may be changed after creation — gates both Update and Patch.
	Mutable
)

// capsAll is the full permissive capability set, the starting point for a plain
// scalar column before exceptions and tag flags are applied.
const capsAll = Filterable | Sortable | Queryable | Creatable | Mutable

// defaultOps returns the default set of filter operators allowed for a DataType.
// It is nil for types that are not filterable by default (relation). The set is
// returned fresh each call so callers may retain it without aliasing.
func defaultOps(t DataType) []FilterOperatorSpec {
	switch t {
	case TypeString:
		return []FilterOperatorSpec{
			OperatorEq, OperatorNe, OperatorIn, OperatorNotIn,
			OperatorLike, OperatorNotLike, OperatorILike, OperatorExists,
		}
	case TypeInt, TypeFloat, TypeTime:
		return []FilterOperatorSpec{
			OperatorEq, OperatorNe, OperatorIn, OperatorNotIn, OperatorExists,
			OperatorGt, OperatorGte, OperatorLt, OperatorLte,
			OperatorBetween, OperatorBetweenEx, OperatorBetweenExInc, OperatorBetweenIncEx,
		}
	case TypeBool:
		return []FilterOperatorSpec{OperatorEq, OperatorNe}
	case TypeEnum:
		return []FilterOperatorSpec{OperatorEq, OperatorNe, OperatorIn, OperatorNotIn}
	case TypeJSON:
		return []FilterOperatorSpec{OperatorExists}
	case TypeRel:
		return nil
	default:
		return nil
	}
}
