package history

import (
	"fmt"
	"time"

	"github.com/amberpixels/r3"
)

// Action is the type of mutation an entry records.
type Action string

const (
	// ActionCreate is a new entity insert.
	ActionCreate Action = "create"
	// ActionUpdate is a full entity update.
	ActionUpdate Action = "update"
	// ActionPatch is a partial update (specific fields only).
	ActionPatch Action = "patch"
	// ActionDelete is a delete (or soft-delete).
	ActionDelete Action = "delete"
	// ActionRevert is a revert to a previous version.
	ActionRevert Action = "revert"
	// ActionEvent is a domain/synthetic timeline event, not a field mutation - see
	// [ChangeRecord.EventType].
	ActionEvent Action = "event"
)

// ChangeRecord is one activity-log entry: what happened to an entity at a point in
// time, as field-level diffs. Snapshots are a separate opt-in feature - see
// [SnapshotRule].
type ChangeRecord struct {
	// ID uniquely identifies this record (UUID).
	ID string `json:"id" db:"id,pk" bson:"_id"`

	// RecordType is the entity type name (e.g. "orders"). Derived from the struct
	// type or set explicitly via options.
	RecordType string `json:"record_type" db:"record_type" bson:"record_type"`

	// RecordID is the entity primary key, stringified.
	RecordID string `json:"record_id" db:"record_id" bson:"record_id"`

	// Action is the mutation type.
	Action Action `json:"action" db:"action" bson:"action"`

	// ActorID identifies who made the change (resolved from r3.GetActor(ctx)). A
	// first-class indexable column, not part of Metadata, so the log can be
	// filtered/grouped by actor. Empty for the system/anonymous actor.
	ActorID string `json:"actor_id,omitempty" db:"actor_id" bson:"actor_id,omitempty"`

	// ActorType categorizes the actor (e.g. "user", "system"); first-class column
	// alongside ActorID.
	ActorType string `json:"actor_type,omitempty" db:"actor_type" bson:"actor_type,omitempty"`

	// Version is a monotonic counter per (record_type, record_id); version 1 is
	// always the create. Enables ordered history and revert-to-version.
	Version int64 `json:"version" db:"version" bson:"version"`

	// Changes is the field-level diff: create -> every field with OldValue=nil;
	// delete -> every field with NewValue=nil; update/patch -> only changed fields;
	// revert -> pre-revert vs reverted-to. Reconstruction ([Reverter.Reconstruct])
	// works purely from diffs because v1 always carries every field. Stored as JSON.
	Changes r3.JSONColumn[[]FieldChange] `json:"changes" db:"changes" bson:"changes"`

	// ParentType is the parent entity's record type (for tree queries); empty if
	// no parent.
	ParentType string `json:"parent_type,omitempty" db:"parent_type" bson:"parent_type,omitempty"`

	// ParentID is the parent primary key, stringified; empty if no parent.
	ParentID string `json:"parent_id,omitempty" db:"parent_id" bson:"parent_id,omitempty"`

	// EventType, when non-empty, marks this entry as a timeline event (Action is
	// ActionEvent) rather than a field mutation - e.g. "comment_added". Changes is
	// usually empty; EventData carries the payload. Changes and events share one
	// ordered timeline.
	EventType string `json:"event_type,omitempty" db:"event_type" bson:"event_type,omitempty"`

	// EventData is an optional structured event payload. Stored as JSON.
	EventData r3.JSONColumn[map[string]any] `json:"event_data" db:"event_data" bson:"event_data,omitempty"`

	// Synthetic marks a record reconstructed rather than observed - e.g. a
	// backfilled create inferred from an entity's CURRENT state because tracking
	// began after it existed. Its Changes are best-effort, not the original values.
	Synthetic bool `json:"synthetic,omitempty" db:"synthetic" bson:"synthetic,omitempty"`

	// Note is an optional human remark (e.g. why a record is synthetic).
	Note string `json:"note,omitempty" db:"note" bson:"note,omitempty"`

	// Metadata carries surrounding context (source, request correlation). Stored as
	// JSON.
	Metadata r3.JSONColumn[Metadata] `json:"metadata" db:"metadata" bson:"metadata"`

	// CreatedAt is when the change was recorded.
	CreatedAt time.Time `json:"created_at" db:"created_at" bson:"created_at"`
}

// FieldChange is a single field-level change. Field uses flat dot-notation (e.g.
// "address.city") matching the storage column names.
type FieldChange struct {
	// Field is the changed column/field name (dot-notation for nested structs).
	Field string `json:"field"`

	// OldValue is the previous value (nil for create).
	OldValue any `json:"old_value"`

	// NewValue is the new value (nil for delete).
	NewValue any `json:"new_value"`
}

// String renders the field change as "field: old -> new".
func (fc FieldChange) String() string {
	return fmt.Sprintf("%s: %v -> %v", fc.Field, fc.OldValue, fc.NewValue)
}

// Metadata carries context surrounding a change (not the actor, which is
// first-class on ChangeRecord). Extracted from context via a user MetadataFunc.
type Metadata struct {
	// Source is where the change originated (e.g. "api", "admin_ui", "worker").
	Source string `json:"source,omitempty"`

	// Extra holds arbitrary context pairs (e.g. request_id, ip_address).
	Extra map[string]string `json:"extra,omitempty"`
}

// TreeScope scopes a history query across a parent-child hierarchy. Used with
// QueryForTree to retrieve changes for a record and all its descendants.
//
//	[]TreeScope{
//	    {RecordType: "cities", RecordID: "5"},
//	    {RecordType: "locations", ParentType: "cities", ParentID: "5"},
//	}
type TreeScope struct {
	// RecordType is the entity type to match.
	RecordType string

	// RecordID limits to one instance; empty means all of this type.
	RecordID string

	// ParentType scopes to children of a parent type; empty means a root scope.
	ParentType string

	// ParentID scopes to children of a parent instance; empty with a set
	// ParentType means all children of that type.
	ParentID string
}
