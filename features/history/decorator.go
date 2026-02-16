package history

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/amberpixels/r3"
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
		inner: inner,
		store: store,
		opts:  opts,
	}
}

// Inner returns the underlying CRUD repository (unwrapped).
// Useful when you need to bypass history tracking for a specific operation.
func (h *CRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return h.inner
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
		crud:  h.inner,
		store: h.store,
		opts:  h.opts,
	}
}

// Create inserts a new entity and records a "create" change.
func (h *CRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
	result, err := h.inner.Create(ctx, entity)
	if err != nil {
		return result, err
	}

	var zero T
	h.record(ctx, recordInfo[T]{entity: result, old: zero, hasOld: false, action: ActionCreate}, func() []FieldChange {
		return DiffCreate(result)
	})

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

	h.record(ctx, recordInfo[T]{entity: result, old: old, hasOld: hasOld, action: ActionUpdate}, func() []FieldChange {
		if hasOld {
			return h.opts.DiffFunc(old, result)
		}
		return nil
	})

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

	h.record(ctx, recordInfo[T]{entity: result, old: old, hasOld: hasOld, action: ActionPatch}, func() []FieldChange {
		if hasOld {
			fieldNames := r3.FieldsToStrings(fields)
			return DiffWithFields(old, result, fieldNames)
		}
		return nil
	})

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
		h.record(ctx, recordInfo[T]{entity: old, old: old, hasOld: true, action: ActionDelete}, func() []FieldChange {
			return DiffDelete(old)
		})
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

// record creates and persists a ChangeRecord. If Async is enabled, it runs
// in a background goroutine. Errors during async recording are logged.
//
// After recording, snapshot rules are evaluated — snapshots are stored
// separately via each rule's snapshot CRUD.
func (h *CRUD[T, ID]) record(ctx context.Context, info recordInfo[T], diffFn func() []FieldChange) {
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

		// Metadata
		if h.opts.MetadataFunc != nil {
			record.Metadata = r3.NewJSONColumn(h.opts.MetadataFunc(ctx))
		}

		// Parent reference
		if h.opts.ParentRef != nil {
			record.ParentType = h.opts.ParentRef.ParentType
			record.ParentID = extractFieldByName(info.entity, h.opts.ParentRef.FKField)
		}

		return record
	}

	if h.opts.Async {
		go func() {
			record := buildRecord()
			// Compute next version internally
			record.Version = nextVersion(context.Background(), h.store, record.RecordType, record.RecordID)
			record.ID = generateID()

			if _, err := h.store.Create(context.Background(), record); err != nil {
				slog.ErrorContext(
					context.Background(),
					"r3history: async record failed",
					"record_type",
					record.RecordType,
					"record_id",
					record.RecordID,
					"error",
					err,
				)
			}
			// Evaluate snapshot rules
			if len(h.opts.SnapshotRules) > 0 {
				evaluateSnapshotRules(
					context.Background(),
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
		}()
		return
	}

	record := buildRecord()
	// Compute next version internally
	record.Version = nextVersion(ctx, h.store, record.RecordType, record.RecordID)
	record.ID = generateID()

	if _, err := h.store.Create(ctx, record); err != nil {
		slog.ErrorContext(
			ctx,
			"r3history: record failed",
			"record_type",
			record.RecordType,
			"record_id",
			record.RecordID,
			"error",
			err,
		)
	}

	// Evaluate snapshot rules
	if len(h.opts.SnapshotRules) > 0 {
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

// generateID creates a unique ID for a change record.
func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
