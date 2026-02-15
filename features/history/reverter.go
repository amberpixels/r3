package history

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/amberpixels/r3"
)

// Reverter provides revert/undo capabilities for a history-tracked entity.
//
// It reconstructs historical states purely from field-level diffs and applies
// them via the inner CRUD's Update method. Every revert is itself recorded as
// a new change record (Action: "revert"), so the audit trail is always
// append-only and reverting a revert is possible.
//
// State reconstruction strategy:
//   - Start from version 1 (the create record, which contains every field
//     as a FieldChange with OldValue=nil), then replay changes forward to
//     the target version.
//   - No snapshots are needed — this is purely diff-based.
type Reverter[T any, ID comparable] struct {
	crud  r3.CRUD[T, ID]
	store Store
	opts  Options[T, ID]
}

// RevertTo restores an entity to the state it was in at a specific version.
//
// It reconstructs the state by replaying all FieldChanges from version 1
// through the target version. A new change record with Action "revert"
// is always created.
//
// Returns the restored entity and an error if reconstruction fails.
func (r *Reverter[T, ID]) RevertTo(ctx context.Context, id ID, version int64) (T, error) {
	var zero T

	recordID := fmt.Sprint(id)

	// Reconstruct the state at the target version from diffs.
	restored, err := r.Reconstruct(ctx, recordID, version)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to reconstruct version %d: %w", version, err)
	}

	// Fetch current state for the diff
	current, err := r.crud.Get(ctx, id)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to get current state for diff: %w", err)
	}

	// Apply the reverted state via Update
	result, err := r.crud.Update(ctx, restored)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to apply reverted state: %w", err)
	}

	// Record the revert as a new change
	var diffFn DiffFunc[T]
	if r.opts.DiffFunc != nil {
		diffFn = r.opts.DiffFunc
	} else {
		diffFn = Diff[T]
	}

	record := ChangeRecord{
		RecordType: r.opts.RecordType,
		RecordID:   recordID,
		Action:     ActionRevert,
		Changes:    diffFn(current, result),
		CreatedAt:  time.Now(),
	}

	if r.opts.MetadataFunc != nil {
		record.Metadata = r.opts.MetadataFunc(ctx)
		record.Metadata.Extra = mergeExtra(record.Metadata.Extra, map[string]string{
			"reverted_to_version": strconv.FormatInt(version, 10),
		})
	} else {
		record.Metadata = Metadata{
			Extra: map[string]string{
				"reverted_to_version": strconv.FormatInt(version, 10),
			},
		}
	}

	if r.opts.ParentRef != nil {
		record.ParentType = r.opts.ParentRef.ParentType
		record.ParentID = extractFieldByName(result, r.opts.ParentRef.FKField)
	}

	if v, err := r.store.NextVersion(ctx, record.RecordType, record.RecordID); err == nil {
		record.Version = v
	}

	if err := r.store.Record(ctx, record); err != nil {
		slog.ErrorContext(
			ctx,
			"r3history: failed to record revert",
			"record_type",
			record.RecordType,
			"record_id",
			record.RecordID,
			"error",
			err,
		)
	}

	// Evaluate snapshot rules after revert
	if len(r.opts.SnapshotRules) > 0 {
		meta := record.Metadata
		evaluateSnapshotRules(
			ctx,
			r.opts.SnapshotRules,
			record.RecordType,
			record.RecordID,
			record.Version,
			ActionRevert,
			current,
			result,
			meta,
		)
	}

	return result, nil
}

// RevertLast undoes the most recent change by reverting to the previous version.
// Equivalent to RevertTo(ctx, id, latestVersion-1).
//
// Returns ErrNoHistory if the entity has no history, or an error if there's
// only one version (the initial create — nothing to revert to).
func (r *Reverter[T, ID]) RevertLast(ctx context.Context, id ID) (T, error) {
	var zero T

	recordID := fmt.Sprint(id)

	latest, err := r.store.LatestVersion(ctx, r.opts.RecordType, recordID)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to get latest version: %w", err)
	}

	if latest.Version <= 1 {
		return zero, errors.New("r3history: cannot revert past the initial create (version 1)")
	}

	return r.RevertTo(ctx, id, latest.Version-1)
}

// Reconstruct rebuilds the entity state at a specific version by replaying
// all FieldChanges from version 1 (the create) through the target version.
//
// This is purely diff-based — no snapshots are needed. Version 1 (create)
// always contains every field as a FieldChange (OldValue=nil, NewValue=<value>),
// which provides the baseline. Subsequent versions' changes are applied on top.
//
// Strategy:
//  1. Fetch all records from version 1 to the target version.
//  2. Start with a zero-value T.
//  3. Apply changes from each version in order (v1, v2, ..., vN).
//  4. Return the reconstructed entity.
func (r *Reverter[T, ID]) Reconstruct(ctx context.Context, recordID string, version int64) (T, error) {
	var zero T

	// Fetch all records up to the target version
	records, _, err := r.store.ForRecord(ctx, r.opts.RecordType, recordID)
	if err != nil {
		return zero, fmt.Errorf("failed to fetch history: %w", err)
	}

	if len(records) == 0 {
		return zero, fmt.Errorf("no history for %s/%s", r.opts.RecordType, recordID)
	}

	// Start with a zero-value entity and replay all changes up to the target version.
	// Version 1 (create) provides all fields, so this builds the complete state.
	entity := zero

	for _, rec := range records {
		if rec.Version > version {
			break
		}
		var err error
		entity, err = applyChanges(entity, rec.Changes)
		if err != nil {
			return zero, fmt.Errorf("failed to apply changes from version %d: %w", rec.Version, err)
		}
	}

	return entity, nil
}

// applyChanges applies a set of FieldChanges to an entity, producing the new state.
// It works by marshaling the entity to a JSON map, applying the changes, then
// unmarshaling back to T. This is backend-agnostic and handles nested dot-notation.
func applyChanges[T any](entity T, changes []FieldChange) (T, error) {
	if len(changes) == 0 {
		return entity, nil
	}

	// Marshal entity to JSON map for manipulation
	data, err := json.Marshal(entity)
	if err != nil {
		return entity, fmt.Errorf("marshal entity: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return entity, fmt.Errorf("unmarshal to map: %w", err)
	}

	// Apply each change
	for _, c := range changes {
		setNestedValue(m, c.Field, c.NewValue)
	}

	// Marshal back to JSON then unmarshal to T
	data, err = json.Marshal(m)
	if err != nil {
		return entity, fmt.Errorf("marshal modified map: %w", err)
	}

	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return entity, fmt.Errorf("unmarshal to entity: %w", err)
	}

	return result, nil
}

// setNestedValue sets a value in a nested map using dot-notation keys.
// e.g. setNestedValue(m, "address.city", "NYC") sets m["address"]["city"] = "NYC".
func setNestedValue(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")

	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part: set the value
			// Convert typed values to JSON-compatible form
			current[part] = jsonNormalize(value)
			return
		}

		// Navigate into nested map, creating if needed
		next, ok := current[part]
		if !ok {
			nested := make(map[string]any)
			current[part] = nested
			current = nested
			continue
		}

		nested, ok := next.(map[string]any)
		if !ok {
			// The intermediate path isn't a map — overwrite it
			nested = make(map[string]any)
			current[part] = nested
		}
		current = nested
	}
}

// jsonNormalize converts a Go value to its JSON-round-trip equivalent.
// This ensures consistency when comparing values that went through JSON serialization.
func jsonNormalize(v any) any {
	if v == nil {
		return nil
	}

	// For basic types, return as-is (json.Marshal will handle them)
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return jsonNormalize(rv.Elem().Interface())
	default:
		return v
	}
}

// mergeExtra merges two maps, with b taking precedence over a.
func mergeExtra(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	result := make(map[string]string, len(a)+len(b))
	maps.Copy(result, a)
	maps.Copy(result, b)
	return result
}
