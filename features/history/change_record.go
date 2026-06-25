package history

import (
	"fmt"
	"time"

	"github.com/amberpixels/r3"
)

// Action represents the type of mutation that was performed on an entity.
type Action string

const (
	// ActionCreate indicates a new entity was inserted.
	ActionCreate Action = "create"
	// ActionUpdate indicates a full entity update.
	ActionUpdate Action = "update"
	// ActionPatch indicates a partial update (specific fields only).
	ActionPatch Action = "patch"
	// ActionDelete indicates the entity was deleted (or soft-deleted).
	ActionDelete Action = "delete"
	// ActionRevert indicates the entity was reverted to a previous version.
	ActionRevert Action = "revert"
)

// ChangeRecord represents a single entry in the activity log.
// It captures what happened to an entity at a specific point in time
// as field-level diffs. Snapshots are a separate, opt-in feature
// with their own storage — see SnapshotRule.
//
// ChangeRecord is a first-class r3 entity: it can be stored via any
// r3.CRUD[ChangeRecord, string] implementation (SQL, GORM, MongoDB, etc.).
type ChangeRecord struct {
	// ID is the unique identifier for this change record (UUID).
	ID string `json:"id" db:"id,pk" bson:"_id"`

	// RecordType is the type name of the entity (e.g. "orders", "campaigns").
	// Derived automatically from the struct type or set explicitly via options.
	RecordType string `json:"record_type" db:"record_type" bson:"record_type"`

	// RecordID is the primary key of the entity, stringified.
	RecordID string `json:"record_id" db:"record_id" bson:"record_id"`

	// Action is the type of mutation (create, update, patch, delete, revert).
	Action Action `json:"action" db:"action" bson:"action"`

	// ActorID identifies who performed the change (e.g. user ID, API key ID).
	// It is a first-class, indexable column — resolved from r3.GetActor(ctx) —
	// so the activity log can be filtered/grouped by actor ("show everything
	// user 7 did") without reaching into the Metadata JSON blob. Empty for the
	// system/anonymous actor.
	ActorID string `json:"actor_id,omitempty" db:"actor_id" bson:"actor_id,omitempty"`

	// ActorType categorizes the actor (e.g. "user", "system", "api_key", "cron").
	// First-class column alongside ActorID; resolved from r3.GetActor(ctx).
	ActorType string `json:"actor_type,omitempty" db:"actor_type" bson:"actor_type,omitempty"`

	// Version is a monotonically increasing counter per (record_type, record_id).
	// Version 1 is always the initial create. This enables ordered history
	// and "revert to version N" functionality.
	Version int64 `json:"version" db:"version" bson:"version"`

	// Changes contains the field-level diff for this mutation.
	// For create: each field appears with OldValue=nil and NewValue=<value>.
	// For delete: each field appears with OldValue=<value> and NewValue=nil.
	// For update/patch: only changed fields appear.
	// For revert: the diff between pre-revert state and the reverted-to state.
	//
	// State reconstruction (Reverter.Reconstruct) works purely from diffs:
	// version 1 (create) always contains every field, so replaying from v1
	// forward reconstructs any historical state without needing snapshots.
	//
	// Stored as a JSON blob in SQL databases via JSONColumn.
	Changes r3.JSONColumn[[]FieldChange] `json:"changes" db:"changes" bson:"changes"`

	// ParentType is the record type of the parent entity (for tree queries).
	// Empty if this entity has no parent. Example: "campaigns" for an adset.
	ParentType string `json:"parent_type,omitempty" db:"parent_type" bson:"parent_type,omitempty"`

	// ParentID is the primary key of the parent entity, stringified.
	// Empty if this entity has no parent.
	ParentID string `json:"parent_id,omitempty" db:"parent_id" bson:"parent_id,omitempty"`

	// Metadata carries contextual information about who made the change and why.
	// Stored as a JSON blob in SQL databases via JSONColumn.
	Metadata r3.JSONColumn[Metadata] `json:"metadata" db:"metadata" bson:"metadata"`

	// CreatedAt is the timestamp when this change was recorded.
	CreatedAt time.Time `json:"created_at" db:"created_at" bson:"created_at"`
}

// FieldChange represents a single field-level change within a mutation.
// Field names use flat dot-notation (e.g. "address.city") matching the
// storage column/field names derived from struct tags.
type FieldChange struct {
	// Field is the column/field name that changed (dot-notation for nested structs).
	Field string `json:"field"`

	// OldValue is the previous value (nil for create).
	OldValue any `json:"old_value"`

	// NewValue is the new value (nil for delete).
	NewValue any `json:"new_value"`
}

// String returns a human-readable representation of the field change.
func (fc FieldChange) String() string {
	return fmt.Sprintf("%s: %v -> %v", fc.Field, fc.OldValue, fc.NewValue)
}

// Metadata carries optional contextual information about a change beyond who
// performed it. The actor is a first-class concern and lives on ChangeRecord
// directly (ActorID/ActorType); Metadata is for the surrounding context (where
// it came from, request correlation, etc.). It is extracted from a
// context.Context via a user-provided MetadataFunc.
type Metadata struct {
	// Source describes where the change originated (e.g. "api", "admin_ui", "worker").
	Source string `json:"source,omitempty"`

	// Extra holds arbitrary key-value pairs for additional context
	// (e.g. request_id, ip_address, user_agent).
	Extra map[string]string `json:"extra,omitempty"`
}

// TreeScope defines a scope for querying history across a parent-child hierarchy.
// Used with QueryForTree to retrieve changes for a record and all its descendants.
//
// Example: query all changes for Campaign #5 and its adsets:
//
//	[]TreeScope{
//	    {RecordType: "campaigns", RecordID: "5"},
//	    {RecordType: "adsets", ParentType: "campaigns", ParentID: "5"},
//	}
type TreeScope struct {
	// RecordType is the entity type to match (e.g. "campaigns", "adsets").
	RecordType string

	// RecordID limits to a specific entity instance. Empty means all of this type.
	RecordID string

	// ParentType scopes to children of a specific parent type.
	// Empty means this is a root scope (match by RecordType/RecordID only).
	ParentType string

	// ParentID scopes to children of a specific parent instance.
	// Empty with a non-empty ParentType means all children of that parent type.
	ParentID string
}
