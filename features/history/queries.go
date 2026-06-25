package history

import (
	"github.com/amberpixels/r3"
)

// Query builder field names — matching the db/bson tags on ChangeRecord.
var (
	fieldRecordType = r3.NewFieldSpec("record_type")
	fieldRecordID   = r3.NewFieldSpec("record_id")
	fieldVersion    = r3.NewFieldSpec("version")
	fieldCreatedAt  = r3.NewFieldSpec("created_at")
	fieldParentType = r3.NewFieldSpec("parent_type")
	fieldParentID   = r3.NewFieldSpec("parent_id")
	fieldActorID    = r3.NewFieldSpec("actor_id")
)

// QueryForRecord builds a Query that filters change records for a specific
// entity instance, sorted by version ascending.
//
// Equivalent to the old Store.ForRecord(ctx, recordType, recordID).
func QueryForRecord(recordType, recordID string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortAscSpec(fieldVersion)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForType builds a Query that filters change records for all entities
// of a given type, sorted by created_at descending.
//
// Equivalent to the old Store.ForType(ctx, recordType).
func QueryForType(recordType string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.F(fieldRecordType, recordType),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForActor builds a Query that filters change records by the actor who
// performed them, sorted by created_at descending. This answers "show me
// everything user X did" — possible only because the actor is a first-class
// column on ChangeRecord (not buried in the Metadata JSON blob).
func QueryForActor(actorID string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.F(fieldActorID, actorID),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForTree builds a Query that matches change records across a parent-child
// hierarchy. Each TreeScope defines one level; they are combined with OR.
//
// Equivalent to the old Store.ForTree(ctx, scopes).
func QueryForTree(scopes []TreeScope) r3.Query {
	orFilters := make(r3.Filters, 0, len(scopes))
	for _, s := range scopes {
		conditions := r3.Filters{r3.F(fieldRecordType, s.RecordType)}

		if s.RecordID != "" {
			conditions = append(conditions, r3.F(fieldRecordID, s.RecordID))
		}
		if s.ParentType != "" {
			conditions = append(conditions, r3.F(fieldParentType, s.ParentType))
		}
		if s.ParentID != "" {
			conditions = append(conditions, r3.F(fieldParentID, s.ParentID))
		}

		// Combine this scope's conditions with AND, then add to OR group
		orFilters = append(orFilters, r3.And(conditions...))
	}

	return r3.Query{
		Filters:    r3.Filters{r3.Or(orFilters...)},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForVersion builds a Query that matches a specific version of a specific entity.
//
// Equivalent to the old Store.GetVersion(ctx, recordType, recordID, version).
func QueryForVersion(recordType, recordID string, version int64) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
				r3.F(fieldVersion, version),
			),
		},
		Pagination: r3.NoPagination(),
	}
}

// QueryLatestVersion builds a Query that returns the most recent change record
// for a specific entity (sort by version desc, limit 1).
//
// Used internally to compute NextVersion and to implement RevertLast.
func QueryLatestVersion(recordType, recordID string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldVersion)},
		Pagination: r3.NewPaginationSpec(1, 1),
	}
}

// QuerySnapshotForVersion builds a Query for retrieving a snapshot at a specific version.
func QuerySnapshotForVersion(recordType, recordID string, version int64) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
				r3.F(fieldVersion, version),
			),
		},
		Pagination: r3.NoPagination(),
	}
}

// QueryLatestSnapshot builds a Query for the most recent snapshot of an entity.
func QueryLatestSnapshot(recordType, recordID string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldVersion)},
		Pagination: r3.NewPaginationSpec(1, 1),
	}
}

// QueryListSnapshots builds a Query that lists all snapshots for a specific entity,
// ordered by version descending.
func QueryListSnapshots(recordType, recordID string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.And(
				r3.F(fieldRecordType, recordType),
				r3.F(fieldRecordID, recordID),
			),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldVersion)},
		Pagination: r3.NoPagination(),
	}
}
