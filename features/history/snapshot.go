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

// Snapshot is a full point-in-time capture of an entity, separate from change
// records: its own storage, triggered only when a [SnapshotRule] condition is met
// (unlike change records, written on every mutation).
type Snapshot struct {
	// ID uniquely identifies this snapshot.
	ID string `json:"id" db:"id,pk" bson:"_id"`

	// RecordType is the entity type name.
	RecordType string `json:"record_type" db:"record_type" bson:"record_type"`

	// RecordID is the entity primary key, stringified.
	RecordID string `json:"record_id" db:"record_id" bson:"record_id"`

	// Version is the change-record version that triggered this snapshot.
	Version int64 `json:"version" db:"version" bson:"version"`

	// RuleName is which SnapshotRule fired (disambiguates multiple rules per type).
	RuleName string `json:"rule_name,omitempty" db:"rule_name" bson:"rule_name,omitempty"`

	// ActorID / ActorType mirror the triggering change record.
	ActorID   string `json:"actor_id,omitempty"   db:"actor_id"   bson:"actor_id,omitempty"`
	ActorType string `json:"actor_type,omitempty" db:"actor_type" bson:"actor_type,omitempty"`

	// Data is the full entity state at snapshot time, JSON bytes for portability.
	Data json.RawMessage `json:"data" db:"data" bson:"data"`

	// Metadata mirrors the change record's context. Stored as JSON.
	Metadata r3.JSONColumn[Metadata] `json:"metadata" db:"metadata" bson:"metadata"`

	// CreatedAt is when the snapshot was taken.
	CreatedAt time.Time `json:"created_at" db:"created_at" bson:"created_at"`
}

// SnapshotConditionFunc reports whether to snapshot after a mutation. old is zero
// for create, cur is zero for delete.
//
//	func(action Action, old, cur BenefitSheet) bool {
//	    return old.Status == "draft" && cur.Status == "published"
//	}
type SnapshotConditionFunc[T any] func(action Action, old, cur T) bool

// SnapshotRule defines when and where to snapshot an entity type; multiple rules
// may exist per entity.
type SnapshotRule[T any] struct {
	// Name identifies this rule (logging and Snapshot.RuleName).
	Name string

	// Store persists snapshots from this rule. Only [r3.Commander] is required -
	// snapshots are write-only here.
	Store r3.Commander[Snapshot, string]

	// Condition gates the snapshot; nil disables the rule.
	Condition SnapshotConditionFunc[T]
}

// MarshalSnapshot serializes an entity to JSON bytes, returning nil if entity is
// nil or marshaling fails.
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

// evaluateSnapshotRules takes a snapshot for every rule whose condition holds.
// Called by the decorator after recording a change.
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

		// Deletes snapshot the pre-deletion state; everything else the post-mutation state.
		var entity any
		if action == ActionDelete {
			entity = old
		} else {
			entity = cur
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
			// Log but don't fail the mutation - snapshots are best-effort add-ons.
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
