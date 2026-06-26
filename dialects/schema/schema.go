package r3schema

import (
	"encoding/json"
	"fmt"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/dialects/canonical"
)

// Version is the schema-JSON contract version. Bump it on a breaking change to
// the emitted shape so consumers can detect and adapt.
const Version = 1

// jsonSchema is the top-level serialized shape.
type jsonSchema struct {
	Version    int             `json:"version"`
	Attributes []jsonAttribute `json:"attributes"`
}

// jsonCaps is the boolean projection of an attribute's capability bitset.
type jsonCaps struct {
	Filterable bool `json:"filterable"`
	Sortable   bool `json:"sortable"`
	Queryable  bool `json:"queryable"`
	Creatable  bool `json:"creatable"`
	Mutable    bool `json:"mutable"`
}

// jsonRelation describes a relation attribute's target.
type jsonRelation struct {
	Target string `json:"target"`
	Kind   string `json:"kind,omitempty"`
	Label  string `json:"label,omitempty"`
}

// jsonAttribute is the serialized shape of one public attribute.
type jsonAttribute struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Caps     jsonCaps      `json:"caps"`
	Ops      []string      `json:"ops,omitempty"`
	Enum     []string      `json:"enum,omitempty"`
	Relation *jsonRelation `json:"relation,omitempty"`
	Computed bool          `json:"computed"`
}

// MarshalSchema serializes a schema to the public JSON shape: a version plus the
// ordered list of Queryable attributes, each with its capability booleans,
// allowed filter operators (canonical names), enum values, relation target, and
// the reserved computed flag. Non-queryable attributes are omitted so the
// system/worker bypass is never advertised.
func MarshalSchema(s r3.Schema) ([]byte, error) {
	out := jsonSchema{Version: Version, Attributes: marshalAttributes(s)}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("r3schema: marshal: %w", err)
	}
	return data, nil
}

// marshalAttributes builds the public attribute projection.
func marshalAttributes(s r3.Schema) []jsonAttribute {
	attrs := s.Attributes()
	out := make([]jsonAttribute, 0, len(attrs))
	for _, a := range attrs {
		if !a.Has(r3.Queryable) {
			continue
		}
		ja := jsonAttribute{
			Name: a.Name,
			Type: string(a.Type),
			Caps: jsonCaps{
				Filterable: a.Has(r3.Filterable),
				Sortable:   a.Has(r3.Sortable),
				Queryable:  a.Has(r3.Queryable),
				Creatable:  a.Has(r3.Creatable),
				Mutable:    a.Has(r3.Mutable),
			},
			Ops:      operatorNames(a.Ops),
			Enum:     a.Enum,
			Computed: a.Computed,
		}
		if a.Relation != nil {
			ja.Relation = &jsonRelation{
				Target: a.Relation.Target,
				Kind:   a.Relation.Kind,
				Label:  a.Relation.Label,
			}
		}
		out = append(out, ja)
	}
	return out
}

// operatorNames maps r3 operators to their canonical wire names, the same
// vocabulary the serialization dialects use for filters.
func operatorNames(ops []r3.FilterOperatorSpec) []string {
	if len(ops) == 0 {
		return nil
	}
	names := make([]string, 0, len(ops))
	for _, op := range ops {
		names = append(names, canonical.FormatFilterOperator(op))
	}
	return names
}
