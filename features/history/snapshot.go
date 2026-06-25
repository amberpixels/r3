package history

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/amberpixels/r3"
)

// Snapshot represents a full point-in-time capture of an entity.
// Snapshots are completely separate from change records — they have their own
// storage (separate table/collection, e.g. "original_benefit_sheets") and
// are triggered by custom conditions (SnapshotRule).
//
// Unlike change records (which are always created on every mutation),
// snapshots are opt-in and only taken when a SnapshotRule's condition is met.
//
// Snapshot is a first-class r3 entity: it can be stored via any
// r3.CRUD[Snapshot, string] implementation.
type Snapshot struct {
	// ID is the unique identifier for this snapshot.
	ID string `json:"id" db:"id,pk" bson:"_id"`

	// RecordType is the type name of the entity (e.g. "benefit_sheets").
	RecordType string `json:"record_type" db:"record_type" bson:"record_type"`

	// RecordID is the primary key of the entity, stringified.
	RecordID string `json:"record_id" db:"record_id" bson:"record_id"`

	// Version is the change record version that triggered this snapshot.
	Version int64 `json:"version" db:"version" bson:"version"`

	// RuleName identifies which SnapshotRule triggered this snapshot.
	// Useful when multiple rules exist for the same entity type.
	RuleName string `json:"rule_name,omitempty" db:"rule_name" bson:"rule_name,omitempty"`

	// ActorID / ActorType identify who triggered the snapshot, mirrored from the
	// change record that produced it (resolved from r3.GetActor(ctx)).
	ActorID   string `json:"actor_id,omitempty"   db:"actor_id"   bson:"actor_id,omitempty"`
	ActorType string `json:"actor_type,omitempty" db:"actor_type" bson:"actor_type,omitempty"`

	// Data is the full serialized state of the entity at snapshot time.
	// Stored as JSON bytes for portability across storage backends.
	Data json.RawMessage `json:"data" db:"data" bson:"data"`

	// Metadata carries contextual information (same as on the change record).
	// Stored as a JSON blob in SQL databases via JSONColumn.
	Metadata r3.JSONColumn[Metadata] `json:"metadata" db:"metadata" bson:"metadata"`

	// CreatedAt is the timestamp when the snapshot was taken.
	CreatedAt time.Time `json:"created_at" db:"created_at" bson:"created_at"`
}

// SnapshotConditionFunc evaluates whether a snapshot should be taken after a mutation.
// It receives the action, the old state (zero value for create), and the new state
// (zero value for delete). Return true to trigger a snapshot.
//
// Example: snapshot when status changes from "draft" to "published":
//
//	func(action Action, old, cur BenefitSheet) bool {
//	    return old.Status == "draft" && cur.Status == "published"
//	}
type SnapshotConditionFunc[T any] func(action Action, old, cur T) bool

// SnapshotRule defines when and where to take a snapshot for an entity type.
// Multiple rules can exist per entity — each with its own condition and store.
//
// The Store field is any [r3.Commander] for Snapshot — only write access is needed.
//
// Example:
//
//	r3history.SnapshotRule[BenefitSheet]{
//	    Name:  "on_publish",
//	    Store: snapshotCRUD,  // any r3.Commander[Snapshot, string] (or r3.CRUD)
//	    Condition: func(action Action, old, cur BenefitSheet) bool {
//	        return old.Status == "draft" && cur.Status == "published"
//	    },
//	}
type SnapshotRule[T any] struct {
	// Name identifies this rule (for logging and the RuleName field on Snapshot).
	Name string

	// Store is where snapshots triggered by this rule are persisted.
	// Only [r3.Commander] is required — snapshots are only written, never read back
	// through this interface. Any r3.CRUD also satisfies Commander.
	Store r3.Commander[Snapshot, string]

	// Condition determines whether a snapshot should be taken.
	// If nil, the rule never triggers (effectively disabled).
	Condition SnapshotConditionFunc[T]
}

// MarshalSnapshot serializes an entity to JSON bytes for storage in a Snapshot.
// Returns nil if the entity is nil or serialization fails.
func MarshalSnapshot(entity any) json.RawMessage {
	if entity == nil {
		return nil
	}
	data, err := json.Marshal(entity)
	if err != nil {
		return nil
	}
	return data
}

// UnmarshalSnapshot deserializes a JSON snapshot back into an entity of type T.
func UnmarshalSnapshot[T any](data json.RawMessage) (T, error) {
	var entity T
	if len(data) == 0 {
		return entity, errors.New("r3history: empty snapshot data")
	}
	if err := json.Unmarshal(data, &entity); err != nil {
		return entity, fmt.Errorf("r3history: failed to unmarshal snapshot: %w", err)
	}
	return entity, nil
}

// evaluateSnapshotRules checks all snapshot rules and takes snapshots where conditions are met.
// Called by the decorator after recording a change. The old/new entities and action
// are passed through to each rule's condition function.
func evaluateSnapshotRules[T any](
	ctx context.Context,
	rules []SnapshotRule[T],
	rec ChangeRecord,
	action Action,
	old, cur T,
) {
	for _, rule := range rules {
		if rule.Condition == nil || rule.Store == nil {
			continue
		}

		if !rule.Condition(action, old, cur) {
			continue
		}

		// Determine which entity state to snapshot
		var entity any
		if action == ActionDelete {
			entity = old // for deletes, snapshot the pre-deletion state
		} else {
			entity = cur // for everything else, snapshot the post-mutation state
		}

		snapshot := Snapshot{
			ID:         generateID(),
			RecordType: rec.RecordType,
			RecordID:   rec.RecordID,
			Version:    rec.Version,
			RuleName:   rule.Name,
			ActorID:    rec.ActorID,
			ActorType:  rec.ActorType,
			Data:       MarshalSnapshot(entity),
			Metadata:   rec.Metadata,
			CreatedAt:  time.Now(),
		}

		if _, err := rule.Store.Create(ctx, snapshot); err != nil {
			// Log but don't fail the mutation — snapshots are best-effort add-ons.
			// In async mode the caller already doesn't see errors.
			slog.ErrorContext(
				ctx,
				"r3history: snapshot rule failed",
				"rule",
				rule.Name,
				"record_type",
				rec.RecordType,
				"record_id",
				rec.RecordID,
				"error",
				err,
			)
		}
	}
}
