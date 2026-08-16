package enginemongo

import "github.com/amberpixels/r3"

// ResolveProjection validates a query's projection against this type and returns
// the query the engine should lower: the same query, minus any attempt to
// exclude the identity field.
//
// Two things the stateless BSON dialect cannot do on its own:
//
// Validating that an excluded name matches a field this type actually stores.
// Names come from `bson` tags, which [r3.SchemaOf] does not read, so the meta is
// the only authority here.
//
// Dropping an exclusion of the identity field. The dialect refuses to exclude
// "_id", but a model can name its identity otherwise (`bson:"code" r3:"pk"`),
// and an entity without its identity cannot be patched, deleted, or linked. The
// meta knows which field that is.
func (m *StructMeta) ResolveProjection(q r3.Query) (r3.Query, error) {
	if err := q.ValidateProjectionAgainst(m.Fields); err != nil {
		return q, err
	}
	if len(q.ExcludeFields) == 0 {
		return q, nil
	}

	kept := make(r3.Fields, 0, len(q.ExcludeFields))
	for _, f := range q.ExcludeFields {
		if f.String() == m.IDField {
			continue
		}
		kept = append(kept, f)
	}
	q.ExcludeFields = kept
	return q, nil
}
