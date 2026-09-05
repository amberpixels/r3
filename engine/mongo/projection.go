package enginemongo

import (
	"slices"

	"github.com/amberpixels/r3"
)

// ResolveProjection validates a query's projection against this type and returns
// the query the engine should lower: the same query, with the identity field put
// back into whichever projection form would have dropped it.
//
// Two things the stateless BSON dialect cannot do on its own:
//
// Validating that an excluded name matches a field this type actually stores.
// Names come from `bson` tags, which [r3.SchemaOf] does not read, so the meta is
// the only authority here.
//
// Keeping the identity field through either form. The dialect always includes
// "_id" and never excludes it, but a model can name its identity otherwise
// (`bson:"code" r3:"pk"`), and an entity without its identity cannot be patched,
// deleted, or linked. The meta knows which field that is.
func (m *StructMeta) ResolveProjection(q r3.Query) (r3.Query, error) {
	if err := q.ValidateProjectionAgainst(m.Fields); err != nil {
		return q, err
	}

	switch {
	case len(q.Fields) > 0:
		if slices.Contains(r3.FieldsToStrings(q.Fields), m.IDField) {
			return q, nil
		}
		// Copy rather than append in place: q is a value, but its Fields slice
		// shares a backing array with the caller's query.
		kept := make(r3.Fields, 0, len(q.Fields)+1)
		kept = append(kept, q.Fields...)
		kept = append(kept, r3.NewFieldSpec(m.IDField))
		q.Fields = kept

	case len(q.ExcludeFields) > 0:
		kept := make(r3.Fields, 0, len(q.ExcludeFields))
		for _, f := range q.ExcludeFields {
			if f.String() == m.IDField {
				continue
			}
			kept = append(kept, f)
		}
		q.ExcludeFields = kept
	}

	return q, nil
}
