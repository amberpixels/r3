package r3history

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Snapshot represents a full point-in-time capture of an entity.
// Snapshots are completely separate from change records — they have their own
// storage (separate table/collection, e.g. "original_benefit_sheets") and
// are triggered by custom conditions (SnapshotRule).
//
// Unlike change records (which are always created on every mutation),
// snapshots are opt-in and only taken when a SnapshotRule's condition is met.
type Snapshot struct {
	// ID is the unique identifier for this snapshot.
	ID string `json:"id"`

	// RecordType is the type name of the entity (e.g. "benefit_sheets").
	RecordType string `json:"record_type"`

	// RecordID is the primary key of the entity, stringified.
	RecordID string `json:"record_id"`

	// Version is the change record version that triggered this snapshot.
	Version int64 `json:"version"`

	// RuleName identifies which SnapshotRule triggered this snapshot.
	// Useful when multiple rules exist for the same entity type.
	RuleName string `json:"rule_name,omitempty"`

	// Data is the full serialized state of the entity at snapshot time.
	// Stored as JSON bytes for portability across storage backends.
	Data json.RawMessage `json:"data"`

	// Metadata carries contextual information (same as on the change record).
	Metadata Metadata `json:"metadata"`

	// CreatedAt is the timestamp when the snapshot was taken.
	CreatedAt time.Time `json:"created_at"`
}

// SnapshotStore is the backend-agnostic interface for persisting and querying snapshots.
// Implementations are separate from the change record Store — snapshots can live
// in a completely different database, table, or collection.
//
// Example storage layouts:
//   - SQL: "original_benefit_sheets" table alongside "benefit_sheets"
//   - Mongo: "benefit_sheet_snapshots" collection
//   - S3: one object per snapshot for large entities
type SnapshotStore interface {
	// SaveSnapshot persists a snapshot.
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error

	// GetSnapshot retrieves the snapshot for a specific entity at a specific version.
	GetSnapshot(ctx context.Context, recordType string, recordID string, version int64) (Snapshot, error)

	// ListSnapshots returns all snapshots for a specific entity, ordered by version descending.
	ListSnapshots(ctx context.Context, recordType string, recordID string) ([]Snapshot, error)

	// LatestSnapshot returns the most recent snapshot for a specific entity.
	LatestSnapshot(ctx context.Context, recordType string, recordID string) (Snapshot, error)
}

// SnapshotConditionFunc evaluates whether a snapshot should be taken after a mutation.
// It receives the action, the old state (zero value for create), and the new state
// (zero value for delete). Return true to trigger a snapshot.
//
// Example: snapshot when status changes from "draft" to "published":
//
//	func(action Action, old, new BenefitSheet) bool {
//	    return old.Status == "draft" && new.Status == "published"
//	}
type SnapshotConditionFunc[T any] func(action Action, old, new T) bool

// SnapshotRule defines when and where to take a snapshot for an entity type.
// Multiple rules can exist per entity — each with its own condition and store.
//
// Example:
//
//	r3history.SnapshotRule[BenefitSheet]{
//	    Name:  "on_publish",
//	    Store: snapshotStore,
//	    Condition: func(action Action, old, new BenefitSheet) bool {
//	        return old.Status == "draft" && new.Status == "published"
//	    },
//	}
type SnapshotRule[T any] struct {
	// Name identifies this rule (for logging and the RuleName field on Snapshot).
	Name string

	// Store is where snapshots triggered by this rule are persisted.
	Store SnapshotStore

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
		return entity, fmt.Errorf("r3history: empty snapshot data")
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
	recordType string,
	recordID string,
	version int64,
	action Action,
	old, new T,
	meta Metadata,
) {
	for _, rule := range rules {
		if rule.Condition == nil || rule.Store == nil {
			continue
		}

		if !rule.Condition(action, old, new) {
			continue
		}

		// Determine which entity state to snapshot
		var entity any
		if action == ActionDelete {
			entity = old // for deletes, snapshot the pre-deletion state
		} else {
			entity = new // for everything else, snapshot the post-mutation state
		}

		snapshot := Snapshot{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			RecordType: recordType,
			RecordID:   recordID,
			Version:    version,
			RuleName:   rule.Name,
			Data:       MarshalSnapshot(entity),
			Metadata:   meta,
			CreatedAt:  time.Now(),
		}

		if err := rule.Store.SaveSnapshot(ctx, snapshot); err != nil {
			// Log but don't fail the mutation — snapshots are best-effort add-ons.
			// In async mode the caller already doesn't see errors.
			fmt.Printf("r3history: snapshot rule %q failed for %s/%s: %v\n", rule.Name, recordType, recordID, err)
		}
	}
}
