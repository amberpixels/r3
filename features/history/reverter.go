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

// Reverter provides revert/undo for a history-tracked entity. It reconstructs
// historical state purely from field diffs (replaying v1 forward - no snapshots)
// and applies it via the inner CRUD's Update. Every revert is itself recorded as
// an ActionRevert change, so the trail stays append-only and a revert is
// revertible.
type Reverter[T any, ID comparable] struct {
	crud  r3.CRUD[T, ID]
	store r3.CRUD[ChangeRecord, string]
	opts  Options[T, ID]

	// versionLocks is shared with the owning history CRUD so revert records
	// participate in the same per-record version serialization.
	versionLocks *versionLocker
}

// handleError reports a failed revert change-record write via the configured
// ErrorHandler, or slog if none is set.
func (r *Reverter[T, ID]) handleError(ctx context.Context, err error) {
	if r.opts.ErrorHandler != nil {
		r.opts.ErrorHandler(err)
		return
	}
	slog.ErrorContext(ctx, "r3history: revert record failed", "record_type", r.opts.RecordType, "error", err)
}

// RevertTo restores an entity to its state at a specific version (reconstructed
// from diffs) and always records an ActionRevert change.
func (r *Reverter[T, ID]) RevertTo(ctx context.Context, id ID, version int64) (T, error) {
	var zero T

	recordID := fmt.Sprint(id)

	restored, err := r.Reconstruct(ctx, recordID, version)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to reconstruct version %d: %w", version, err)
	}

	// Fetch current state for the diff.
	current, err := r.crud.Get(ctx, id)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to get current state for diff: %w", err)
	}

	result, err := r.crud.Update(ctx, restored)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to apply reverted state: %w", err)
	}

	var diffFn DiffFunc[T]
	if r.opts.DiffFunc != nil {
		diffFn = r.opts.DiffFunc
	} else {
		diffFn = Diff[T]
	}

	actor := resolveActor(ctx, r.opts.FixedActor)
	record := ChangeRecord{
		RecordType: r.opts.RecordType,
		RecordID:   recordID,
		Action:     ActionRevert,
		ActorID:    actor.ID,
		ActorType:  actor.Type,
		Changes:    r3.NewJSONColumn(diffFn(current, result)),
		CreatedAt:  time.Now(),
	}

	meta := metadataFromCtx(ctx, r.opts.MetadataFunc)
	meta.Extra = mergeExtra(meta.Extra, map[string]string{
		"reverted_to_version": strconv.FormatInt(version, 10),
	})
	record.Metadata = r3.NewJSONColumn(meta)

	if r.opts.ParentRef != nil {
		record.ParentType = r.opts.ParentRef.ParentType
		record.ParentID = extractFieldByName(result, r.opts.ParentRef.FKField)
	}

	// Assign version + ID and persist atomically per record key.
	record, err = persistVersioned(ctx, r.store, r.versionLocks, record)
	if err != nil {
		r.handleError(ctx, err)
		if r.opts.FailOnError {
			return zero, fmt.Errorf(
				"r3history: failed to record revert of %s/%s: %w",
				record.RecordType, record.RecordID, err,
			)
		}
		// best-effort: the revert mutation already succeeded
		return result, nil
	}

	if len(r.opts.SnapshotRules) > 0 {
		evaluateSnapshotRules(
			ctx,
			r.opts.SnapshotRules,
			record,
			ActionRevert,
			current,
			result,
		)
	}

	return result, nil
}

// RevertLast undoes the most recent change (RevertTo latestVersion-1). Returns
// ErrNoHistory if there is no history, or an error if only the initial create
// (version 1) exists.
func (r *Reverter[T, ID]) RevertLast(ctx context.Context, id ID) (T, error) {
	var zero T

	recordID := fmt.Sprint(id)

	q := QueryLatestVersion(r.opts.RecordType, recordID)
	records, _, err := r.store.List(ctx, q)
	if err != nil {
		return zero, fmt.Errorf("r3history: failed to get latest version: %w", err)
	}
	if len(records) == 0 {
		return zero, ErrNoHistory
	}

	latest := records[0]
	if latest.Version <= 1 {
		return zero, errors.New("r3history: cannot revert past the initial create (version 1)")
	}

	return r.RevertTo(ctx, id, latest.Version-1)
}

// Reconstruct rebuilds entity state at a version by replaying FieldChanges from v1
// through the target. Purely diff-based: v1 (create) carries every field as the
// baseline, later versions apply on top.
func (r *Reverter[T, ID]) Reconstruct(ctx context.Context, recordID string, version int64) (T, error) {
	var zero T

	q := QueryForRecord(r.opts.RecordType, recordID)
	records, _, err := r.store.List(ctx, q)
	if err != nil {
		return zero, fmt.Errorf("failed to fetch history: %w", err)
	}

	if len(records) == 0 {
		return zero, fmt.Errorf("no history for %s/%s", r.opts.RecordType, recordID)
	}

	// Replay from zero up to the target version; v1 provides every field.
	entity := zero

	for _, rec := range records {
		if rec.Version > version {
			break
		}
		var err error
		entity, err = applyChanges(entity, rec.Changes.Val)
		if err != nil {
			return zero, fmt.Errorf("failed to apply changes from version %d: %w", rec.Version, err)
		}
	}

	return entity, nil
}

// applyChanges applies FieldChanges to an entity via a JSON-map round-trip:
// backend-agnostic and handles nested dot-notation.
func applyChanges[T any](entity T, changes []FieldChange) (T, error) {
	if len(changes) == 0 {
		return entity, nil
	}

	data, err := json.Marshal(entity)
	if err != nil {
		return entity, fmt.Errorf("marshal entity: %w", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return entity, fmt.Errorf("unmarshal to map: %w", err)
	}

	for _, c := range changes {
		setNestedValue(m, c.Field, c.NewValue)
	}

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

// setNestedValue sets a dot-notation path in a nested map, creating intermediate
// maps as needed. e.g. "address.city" sets m["address"]["city"].
func setNestedValue(m map[string]any, path string, value any) {
	parts := strings.Split(path, ".")

	current := m
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = jsonNormalize(value)
			return
		}

		next, ok := current[part]
		if !ok {
			nested := make(map[string]any)
			current[part] = nested
			current = nested
			continue
		}

		nested, ok := next.(map[string]any)
		if !ok {
			// The intermediate path isn't a map - overwrite it.
			nested = make(map[string]any)
			current[part] = nested
		}
		current = nested
	}
}

// jsonNormalize converts a Go value to its JSON-round-trip equivalent, for
// consistent comparison of values that went through JSON serialization.
func jsonNormalize(v any) any {
	if v == nil {
		return nil
	}

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
