package i18n

import (
	"github.com/amberpixels/r3"
)

// Query builder field names - matching the db/bson tags on Translation.
var (
	fieldEntityType = r3.NewFieldSpec("entity_type")
	fieldEntityID   = r3.NewFieldSpec("entity_id")
	fieldField      = r3.NewFieldSpec("field")
	fieldLang       = r3.NewFieldSpec("lang")
	fieldStale      = r3.NewFieldSpec("stale")
)

// QueryFor matches the translations of one entity in one language; pass
// fields to narrow to specific translated fields (none = all fields).
func QueryFor(entityType, entityID, lang string, fields ...string) r3.Query {
	conds := r3.Filters{
		r3.F(fieldEntityType, entityType),
		r3.F(fieldEntityID, entityID),
		r3.F(fieldLang, lang),
	}
	if len(fields) == 1 {
		conds = append(conds, r3.F(fieldField, fields[0]))
	} else if len(fields) > 1 {
		conds = append(conds, r3.In("field", fields))
	}
	return r3.Query{
		Filters:    r3.Filters{r3.And(conds...)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForEntity matches every translation of one entity - all languages,
// all fields. Used by the decorator for staleness marking and delete cleanup,
// and by admin UIs listing an entity's translations.
func QueryForEntity(entityType, entityID string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldEntityType, entityType),
				r3.F(fieldEntityID, entityID),
			),
		},
		Pagination: r3.NoPagination(),
	}
}

// QueryForBatch matches the translations of many entities in one language -
// the single query behind List overlays (never N+1).
func QueryForBatch(entityType, lang string, entityIDs []string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldEntityType, entityType),
				r3.F(fieldLang, lang),
				r3.In("entity_id", entityIDs),
			),
		},
		Pagination: r3.NoPagination(),
	}
}

// QueryStale matches stale translations of one entity type (any language) - a
// worker's re-translation queue. Entities with NO translation yet are invisible
// here; finding those needs an anti-join ("has no translation") that r3's
// relationship filters can't express yet, so applications write that query
// natively for now.
func QueryStale(entityType string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldEntityType, entityType),
				r3.F(fieldStale, true),
			),
		},
		Pagination: r3.NoPagination(),
	}
}
