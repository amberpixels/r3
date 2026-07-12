package history

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/amberpixels/r3"
	"github.com/google/uuid"
)

// CRUD records every mutation on the wrapped repo as a [ChangeRecord] in the
// history store. Reads (Get, List, Count) pass through unrecorded.
type CRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
	store r3.CRUD[ChangeRecord, string]
	opts  Options[T, ID]

	// versionLocks serializes per-record version assignment (see versionLocker).
	versionLocks *versionLocker
}

var _ r3.CRUD[any, any] = &CRUD[any, any]{}
var _ r3.Aggregator = &CRUD[any, any]{}
var _ r3.RelationAggregator = &CRUD[any, any]{}

// WithHistory wraps a repo with history tracking. store is any
// r3.CRUD[ChangeRecord, string]; the IDFunc option is required so the decorator
// can extract an entity's primary key.
//
//	repo := r3history.WithHistory[Order, int64](
//	    gormOrderCRUD,
//	    historyStore,
//	    r3history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
//	    r3history.WithMetadataFunc[Order, int64](metadataFromCtx),
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

// Inner returns the wrapped repo, bypassing history tracking.
func (h *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return h.inner
}

// Unwrap returns the wrapped CRUD for chain walking (capability detection, tx).
func (h *CRUD[T, ID]) Unwrap() r3.CRUD[T, ID] {
	return h.inner
}

// Rewrap re-applies this layer over a different inner CRUD (e.g. a tx-bound one).
// The store, options, and version locks are shared so version assignment stays
// serialized across the original and tx-bound layers.
func (h *CRUD[T, ID]) Rewrap(inner r3.CRUD[T, ID]) r3.CRUD[T, ID] {
	return &CRUD[T, ID]{inner: inner, store: h.store, opts: h.opts, versionLocks: h.versionLocks}
}

// History returns the change-record CRUD, for use with the query builders
// (QueryForRecord, QueryForType, ...).
func (h *CRUD[T, ID]) History() r3.CRUD[ChangeRecord, string] {
	return h.store
}

// Reverter returns a Reverter for undo/revert on this entity type.
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

// Aggregate passes through to the inner CRUD (read, unrecorded).
func (h *CRUD[T, ID]) Aggregate(ctx context.Context, qarg ...r3.Query) ([]r3.AggregateRow, error) {
	agg, ok := h.inner.(r3.Aggregator)
	if !ok {
		return nil, r3.ErrAggregateNotSupported
	}
	return agg.Aggregate(ctx, qarg...)
}

// AggregateThroughRelation passes through to the inner CRUD (read, unrecorded).
func (h *CRUD[T, ID]) AggregateThroughRelation(
	ctx context.Context, relation string, qarg ...r3.Query,
) ([]r3.AggregateRow, error) {
	agg, ok := h.inner.(r3.RelationAggregator)
	if !ok {
		return nil, r3.ErrRelationAggregateNotSupported
	}
	return agg.AggregateThroughRelation(ctx, relation, qarg...)
}

// Update modifies an entity and records an "update" change with field-level diff.
func (h *CRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
	// Fetch old state for diffing.
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
	// Fetch old state for diffing.
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
	// Fetch pre-deletion state for the diff and snapshot rules.
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

// RecordEvent appends a timeline event (Action ActionEvent) to an entity, separate
// from field-level changes. It takes the next timeline version but is skipped by
// state reconstruction (no diff). The actor is resolved like any change.
func (h *CRUD[T, ID]) RecordEvent(
	ctx context.Context, id ID, eventType string, data map[string]any,
) (ChangeRecord, error) {
	actor := resolveActor(ctx, h.opts.FixedActor)
	rec := ChangeRecord{
		RecordType: h.opts.RecordType,
		RecordID:   fmt.Sprint(id),
		Action:     ActionEvent,
		EventType:  eventType,
		EventData:  r3.NewJSONColumn(data),
		ActorID:    actor.ID,
		ActorType:  actor.Type,
		Metadata:   r3.NewJSONColumn(metadataFromCtx(ctx, h.opts.MetadataFunc)),
		CreatedAt:  time.Now(),
	}
	return persistVersioned(ctx, h.store, h.versionLocks, rec)
}

// RecordSyntheticCreate writes a best-effort "create" for an entity that predates
// tracking: it captures the CURRENT state as the create diff, dates it `at`, and
// flags the record Synthetic. Use during backfill so the entity has an origin point.
func (h *CRUD[T, ID]) RecordSyntheticCreate(
	ctx context.Context, entity T, at time.Time,
) (ChangeRecord, error) {
	actor := resolveActor(ctx, h.opts.FixedActor)
	recordID := ""
	if h.opts.IDFunc != nil {
		recordID = fmt.Sprint(h.opts.IDFunc(entity))
	}
	rec := ChangeRecord{
		RecordType: h.opts.RecordType,
		RecordID:   recordID,
		Action:     ActionCreate,
		Synthetic:  true,
		Note:       "Reconstructed from current data - the values at creation are not known.",
		Changes:    r3.NewJSONColumn(DiffCreate(entity)),
		ActorID:    actor.ID,
		ActorType:  actor.Type,
		Metadata:   r3.NewJSONColumn(metadataFromCtx(ctx, h.opts.MetadataFunc)),
		CreatedAt:  at,
	}
	return persistVersioned(ctx, h.store, h.versionLocks, rec)
}

// recordInfo carries state for building a change record and evaluating snapshots.
type recordInfo[T any] struct {
	entity T      // the entity after mutation (or before deletion)
	old    T      // the entity before mutation (zero value for create)
	hasOld bool   // whether old is valid
	action Action // the mutation type
}

// record builds and persists a ChangeRecord, then evaluates snapshot rules. In
// Async mode it runs in a background goroutine and always returns nil (errors go
// to handleError). Synchronously, a failed write goes to handleError and is
// returned only if FailOnError is set; otherwise recording is best-effort.
func (h *CRUD[T, ID]) record(ctx context.Context, info recordInfo[T], diffFn func() []FieldChange) error {
	buildRecord := func() ChangeRecord {
		recordID := ""
		if h.opts.IDFunc != nil {
			recordID = fmt.Sprint(h.opts.IDFunc(info.entity))
		}

		actor := resolveActor(ctx, h.opts.FixedActor)
		record := ChangeRecord{
			RecordType: h.opts.RecordType,
			RecordID:   recordID,
			Action:     info.action,
			ActorID:    actor.ID,
			ActorType:  actor.Type,
			Changes:    r3.NewJSONColumn(diffFn()),
			CreatedAt:  time.Now(),
		}

		// Metadata is surrounding context only; the actor is first-class above.
		if h.opts.MetadataFunc != nil {
			record.Metadata = r3.NewJSONColumn(h.opts.MetadataFunc(ctx))
		}

		if h.opts.ParentRef != nil {
			record.ParentType = h.opts.ParentRef.ParentType
			record.ParentID = extractFieldByName(info.entity, h.opts.ParentRef.FKField)
		}

		return record
	}

	if h.opts.Async {
		// Detach from the request lifetime so the write isn't cancelled on return,
		// while preserving request-scoped values like the Actor.
		asyncCtx := context.WithoutCancel(ctx)
		go func() {
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
		record,
		info.action,
		info.old,
		info.entity,
	)
}

// handleError reports a failed change-record write via the configured ErrorHandler,
// or slog if none is set.
func (h *CRUD[T, ID]) handleError(ctx context.Context, err error) {
	if h.opts.ErrorHandler != nil {
		h.opts.ErrorHandler(err)
		return
	}
	slog.ErrorContext(ctx, "r3history: record failed", "record_type", h.opts.RecordType, "error", err)
}

// resolveActor attributes a change to the decorator's fixed actor when set (a
// system/worker repo), else the context actor (r3.GetActor, SystemActor by default).
func resolveActor(ctx context.Context, fixed *r3.Actor) r3.Actor {
	if fixed != nil {
		return *fixed
	}
	return r3.GetActor(ctx)
}

// metadataFromCtx returns the Metadata from the user MetadataFunc, or zero if none.
// The actor is resolved separately as a first-class field.
func metadataFromCtx(ctx context.Context, metadataFunc MetadataFunc) Metadata {
	if metadataFunc == nil {
		return Metadata{}
	}
	return metadataFunc(ctx)
}

// nextVersion returns the next version for a (recordType, recordID) pair by
// incrementing the latest recorded version (1 if none).
func nextVersion(ctx context.Context, store r3.CRUD[ChangeRecord, string], recordType, recordID string) int64 {
	q := QueryLatestVersion(recordType, recordID)
	records, _, err := store.List(ctx, q)
	if err != nil || len(records) == 0 {
		return 1
	}
	return records[0].Version + 1
}

// generateID returns a unique change-record ID.
func generateID() string {
	return uuid.NewString()
}
