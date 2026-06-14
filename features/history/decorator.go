package history

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/amberpixels/r3"
	"github.com/google/uuid"
)

// CRUD is a decorator that wraps any r3.CRUD[T, ID] and records every
// mutation as a ChangeRecord in the configured history store.
//
// It transparently satisfies the r3.CRUD[T, ID] interface, so it can be used
// as a drop-in replacement for any CRUD repository. Read operations (Get, List)
// are delegated directly to the inner CRUD without recording anything.
//
// The history store is any r3.CRUD[ChangeRecord, string] — the same CRUD
// abstraction used everywhere in r3. "Everything is a R3po."
//
// Usage:
//
//	repo := r3history.WithHistory[Order, int64](
//	    gormOrderCRUD,
//	    historyStore,  // any r3.CRUD[ChangeRecord, string]
//	    r3history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
//	    r3history.WithMetadataFunc[Order, int64](metadataFromCtx),
//	)
//	// repo satisfies r3.CRUD[Order, int64]
type CRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
	store r3.CRUD[ChangeRecord, string]
	opts  Options[T, ID]

	// versionLocks serializes per-record version assignment (see versionLocker).
	versionLocks *versionLocker
}

// Compile-time check that CRUD satisfies r3.CRUD.
var _ r3.CRUD[any, any] = &CRUD[any, any]{}

// WithHistory wraps an existing r3.CRUD with history tracking.
// The IDFunc option is required — the decorator must know how to extract
// the primary key from an entity.
//
// The store parameter is any r3.CRUD[ChangeRecord, string] — use the same
// CRUD implementation you use for everything else (SQL, GORM, MongoDB, etc.).
//
// Example:
//
//	repo := r3history.WithHistory[Order, int64](
//	    innerRepo, store,
//	    r3history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
//	)
func WithHistory[T any, ID comparable](
	inner r3.CRUD[T, ID],
	store r3.CRUD[ChangeRecord, string],
	optFns ...Option[T, ID],
) *CRUD[T, ID] {
	var opts Options[T, ID]
	for _, fn := range optFns {
		fn(&opts)
	}
	applyDefaults(&opts)

	return &CRUD[T, ID]{
		inner:        inner,
		store:        store,
		opts:         opts,
		versionLocks: newVersionLocker(),
	}
}

// Inner returns the underlying CRUD repository (unwrapped).
// Useful when you need to bypass history tracking for a specific operation.
func (h *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return h.inner
}

// Unwrap returns the wrapped CRUD so capability detection and transaction
// propagation can walk the decorator chain.
func (h *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return h.inner
}

// Rewrap rebuilds this decorator around a different inner CRUD (used to
// re-apply the history layer on top of a transaction-bound CRUD). The change
// store, options, and version locks are shared with the rebuilt instance so
// version assignment stays serialized across the original and tx-bound layers.
func (h *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner, store: h.store, opts: h.opts, versionLocks: h.versionLocks}
}

// History returns the change record CRUD for querying history directly.
// Use it with the query builders (QueryForRecord, QueryForType, etc.)
// to retrieve change records.
func (h *CRUD[T, ID]) History() r3.CRUD[ChangeRecord, string] {
	return h.store
}

// Reverter returns a Reverter for undo/revert operations on this entity type.
func (h *CRUD[T, ID]) Reverter() *Reverter[T, ID] {
	return &Reverter[T, ID]{
		crud:         h.inner,
		store:        h.store,
		opts:         h.opts,
		versionLocks: h.versionLocks,
	}
}

// Create inserts a new entity and records a "create" change.
func (h *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	result, err := h.inner.Create(ctx, entity)
	if err != nil {
		return result, err
	}

	var zero T
	if recErr := h.record(
		ctx,
		recordInfo[T]{entity: result, old: zero, hasOld: false, action: ActionCreate},
		func() []FieldChange {
			return DiffCreate(result)
		},
	); recErr != nil {
		return result, recErr
	}

	return result, nil
}

// Get retrieves an entity by ID. No history is recorded for reads.
func (h *CRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	return h.inner.Get(ctx, id, qarg...)
}

// List retrieves entities. No history is recorded for reads.
func (h *CRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	return h.inner.List(ctx, qarg...)
}

// Count returns the number of matching entities. No history is recorded for reads.
func (h *CRUD[T, ID]) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	return h.inner.Count(ctx, qarg...)
}

// Update modifies an existing entity and records an "update" change with field-level diff.
func (h *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	// Fetch old state for diffing
	var old T
	var hasOld bool
	if h.opts.IDFunc != nil {
		id := h.opts.IDFunc(entity)
		oldEntity, err := h.inner.Get(ctx, id)
		if err == nil {
			old = oldEntity
			hasOld = true
		}
	}

	result, err := h.inner.Update(ctx, entity)
	if err != nil {
		return result, err
	}

	if recErr := h.record(
		ctx,
		recordInfo[T]{entity: result, old: old, hasOld: hasOld, action: ActionUpdate},
		func() []FieldChange {
			if hasOld {
				return h.opts.DiffFunc(old, result)
			}
			return nil
		},
	); recErr != nil {
		return result, recErr
	}

	return result, nil
}

// Patch performs a partial update and records a "patch" change with field-level diff.
func (h *CRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
	// Fetch old state for diffing
	var old T
	var hasOld bool
	if h.opts.IDFunc != nil {
		id := h.opts.IDFunc(entity)
		oldEntity, err := h.inner.Get(ctx, id)
		if err == nil {
			old = oldEntity
			hasOld = true
		}
	}

	result, err := h.inner.Patch(ctx, entity, fields)
	if err != nil {
		return result, err
	}

	if recErr := h.record(
		ctx,
		recordInfo[T]{entity: result, old: old, hasOld: hasOld, action: ActionPatch},
		func() []FieldChange {
			if hasOld {
				fieldNames := r3.FieldsToStrings(fields)
				return DiffWithFields(old, result, fieldNames)
			}
			return nil
		},
	); recErr != nil {
		return result, recErr
	}

	return result, nil
}

// Delete removes an entity and records a "delete" change.
func (h *CRUD[T, ID]) Delete(ctx context.Context, id ID) error {
	// Fetch the entity before deletion for the diff and snapshot rules
	var old T
	var hasOld bool
	oldEntity, err := h.inner.Get(ctx, id)
	if err == nil {
		old = oldEntity
		hasOld = true
	}

	if err := h.inner.Delete(ctx, id); err != nil {
		return err
	}

	if hasOld {
		if recErr := h.record(
			ctx,
			recordInfo[T]{entity: old, old: old, hasOld: true, action: ActionDelete},
			func() []FieldChange {
				return DiffDelete(old)
			},
		); recErr != nil {
			return recErr
		}
	}

	return nil
}

// recordInfo holds context for building a change record and evaluating snapshot rules.
type recordInfo[T any] struct {
	entity T      // the entity after mutation (or before deletion)
	old    T      // the entity before mutation (zero value for create)
	hasOld bool   // whether old is valid
	action Action // the mutation type
}

// record creates and persists a ChangeRecord. If Async is enabled, it runs in a
// background goroutine and always returns nil (errors are reported via
// handleError). In synchronous mode a failed change-record write is reported via
// handleError and, if FailOnError is set, returned to the caller — otherwise nil
// is returned and recording is best-effort.
//
// After a successful record, snapshot rules are evaluated — snapshots are stored
// separately via each rule's snapshot CRUD.
func (h *CRUD[T, ID]) record(ctx context.Context, info recordInfo[T], diffFn func() []FieldChange) error {
	buildRecord := func() ChangeRecord {
		recordID := ""
		if h.opts.IDFunc != nil {
			recordID = fmt.Sprint(h.opts.IDFunc(info.entity))
		}

		record := ChangeRecord{
			RecordType: h.opts.RecordType,
			RecordID:   recordID,
			Action:     info.action,
			Changes:    r3.NewJSONColumn(diffFn()),
			CreatedAt:  time.Now(),
		}

		// Metadata: merge user-provided metadata with Actor from context.
		record.Metadata = r3.NewJSONColumn(buildMetadata(ctx, h.opts.MetadataFunc))

		// Parent reference
		if h.opts.ParentRef != nil {
			record.ParentType = h.opts.ParentRef.ParentType
			record.ParentID = extractFieldByName(info.entity, h.opts.ParentRef.FKField)
		}

		return record
	}

	if h.opts.Async {
		// Detach from the request lifetime (so the write isn't cancelled when the
		// request returns) while preserving request-scoped values like the Actor.
		asyncCtx := context.WithoutCancel(ctx)
		go func() {
			// Assign version + ID and persist atomically per record key.
			record, err := persistVersioned(asyncCtx, h.store, h.versionLocks, buildRecord())
			if err != nil {
				h.handleError(asyncCtx, err)
				return // do not snapshot a change that was not recorded
			}
			h.evaluateSnapshots(asyncCtx, record, info)
		}()
		return nil
	}

	// Assign version + ID and persist atomically per record key.
	record, err := persistVersioned(ctx, h.store, h.versionLocks, buildRecord())
	if err != nil {
		h.handleError(ctx, err)
		if h.opts.FailOnError {
			return fmt.Errorf(
				"r3history: failed to record %s of %s/%s: %w",
				record.Action, record.RecordType, record.RecordID, err,
			)
		}
		return nil // best-effort: the mutation already succeeded
	}

	h.evaluateSnapshots(ctx, record, info)
	return nil
}

// evaluateSnapshots runs the configured snapshot rules for a recorded change.
func (h *CRUD[T, ID]) evaluateSnapshots(ctx context.Context, record ChangeRecord, info recordInfo[T]) {
	if len(h.opts.SnapshotRules) == 0 {
		return
	}
	evaluateSnapshotRules(
		ctx,
		h.opts.SnapshotRules,
		record.RecordType,
		record.RecordID,
		record.Version,
		info.action,
		info.old,
		info.entity,
		record.Metadata.Val,
	)
}

// handleError reports a failed change-record write. It calls the configured
// ErrorHandler if set, otherwise logs via slog.
func (h *CRUD[T, ID]) handleError(ctx context.Context, err error) {
	if h.opts.ErrorHandler != nil {
		h.opts.ErrorHandler(err)
		return
	}
	slog.ErrorContext(ctx, "r3history: record failed", "record_type", h.opts.RecordType, "error", err)
}

// buildMetadata constructs a Metadata value by combining the user-provided
// MetadataFunc (if any) with the r3.Actor from the context. The Actor's ID
// and Type are used as fallback defaults — if the MetadataFunc already sets
// ActorID or ActorType, those values take precedence.
//
// This ensures that r3.WithActor(ctx, actor) automatically populates history
// metadata without requiring an explicit MetadataFunc.
func buildMetadata(ctx context.Context, metadataFunc MetadataFunc) Metadata {
	var meta Metadata
	if metadataFunc != nil {
		meta = metadataFunc(ctx)
	}

	// Fill ActorID/ActorType from context if not already set by MetadataFunc.
	if meta.ActorID == "" || meta.ActorType == "" {
		actor := r3.GetActor(ctx)
		if meta.ActorID == "" {
			meta.ActorID = actor.ID
		}
		if meta.ActorType == "" {
			meta.ActorType = actor.Type
		}
	}

	return meta
}

// nextVersion computes the next version number for a (recordType, recordID) pair
// by querying the latest version and incrementing it.
func nextVersion(ctx context.Context, store r3.CRUD[ChangeRecord, string], recordType, recordID string) int64 {
	q := QueryLatestVersion(recordType, recordID)
	records, _, err := store.List(ctx, q)
	if err != nil || len(records) == 0 {
		return 1
	}
	return records[0].Version + 1
}

// generateID creates a unique, collision-resistant ID for a change record.
func generateID() string {
	return uuid.NewString()
}
