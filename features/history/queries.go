package history

import (
	"github.com/amberpixels/r3"
)

// Query builder field names, matching the db/bson tags on ChangeRecord.
var (
	fieldRecordType = r3.NewFieldSpec("record_type")
	fieldRecordID   = r3.NewFieldSpec("record_id")
	fieldVersion    = r3.NewFieldSpec("version")
	fieldCreatedAt  = r3.NewFieldSpec("created_at")
	fieldParentType = r3.NewFieldSpec("parent_type")
	fieldParentID   = r3.NewFieldSpec("parent_id")
	fieldActorID    = r3.NewFieldSpec("actor_id")
)

// QueryForRecord filters change records for one entity instance, sorted by
// version ascending.
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

// QueryForType filters change records for all entities of a type, sorted by
// created_at descending.
func QueryForType(recordType string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.F(fieldRecordType, recordType),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForActor filters change records by the actor who performed them, sorted
// by created_at descending ("everything user X did") - possible only because the
// actor is a first-class column, not buried in the Metadata JSON blob.
func QueryForActor(actorID string) r3.Query {
	return r3.Query{
		Filters: r3.Filters{
			r3.F(fieldActorID, actorID),
		},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForTree matches change records across a parent-child hierarchy: each
// TreeScope is one level, combined with OR.
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

		orFilters = append(orFilters, r3.And(conditions...))
	}

	return r3.Query{
		Filters:    r3.Filters{r3.Or(orFilters...)},
		Sorts:      r3.Sorts{r3.NewSortDescSpec(fieldCreatedAt)},
		Pagination: r3.NoPagination(),
	}
}

// QueryForVersion matches one specific version of one specific entity.
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

// QueryLatestVersion returns the most recent change record for an entity
// (version desc, limit 1). Used to compute NextVersion and implement RevertLast.
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

// QueryListSnapshots lists all snapshots for an entity, version descending.
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
