package r3history

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/amberpixels/r3"
)

// HistoryCRUD is a decorator that wraps any r3.CRUD[T, ID] and records every
// mutation as a ChangeRecord in the configured Store.
//
// It transparently satisfies the r3.CRUD[T, ID] interface, so it can be used
// as a drop-in replacement for any CRUD repository. Read operations (Get, List)
// are delegated directly to the inner CRUD without recording anything.
//
// Usage:
//
//	repo := r3history.WithHistory[Order, int64](
//	    gormOrderCRUD,
//	    historyStore,
//	    r3history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
//	    r3history.WithMetadataFunc[Order, int64](metadataFromCtx),
//	)
//	// repo satisfies r3.CRUD[Order, int64]
type HistoryCRUD[T any, ID comparable] struct {
	inner r3.CRUD[T, ID]
	store Store
	opts  Options[T, ID]
}

// Compile-time check that HistoryCRUD satisfies r3.CRUD.
var _ r3.CRUD[any, any] = &HistoryCRUD[any, any]{}

// WithHistory wraps an existing r3.CRUD with history tracking.
// The IDFunc option is required — the decorator must know how to extract
// the primary key from an entity.
//
// Example:
//
//	repo := r3history.WithHistory[Order, int64](
//	    innerRepo, store,
//	    r3history.WithIDFunc[Order, int64](func(o Order) int64 { return o.ID }),
//	)
func WithHistory[T any, ID comparable](
	inner r3.CRUD[T, ID],
	store Store,
	optFns ...Option[T, ID],
) *HistoryCRUD[T, ID] {
	var opts Options[T, ID]
	for _, fn := range optFns {
		fn(&opts)
	}
	applyDefaults(&opts)

	return &HistoryCRUD[T, ID]{
		inner: inner,
		store: store,
		opts:  opts,
	}
}

// Inner returns the underlying CRUD repository (unwrapped).
// Useful when you need to bypass history tracking for a specific operation.
func (h *HistoryCRUD[T, ID]) Inner() r3.CRUD[T, ID] {
	return h.inner
}

// History returns the Store for querying change records directly.
func (h *HistoryCRUD[T, ID]) History() Store {
	return h.store
}

// Reverter returns a Reverter for undo/revert operations on this entity type.
func (h *HistoryCRUD[T, ID]) Reverter() *Reverter[T, ID] {
	return &Reverter[T, ID]{
		crud:  h.inner,
		store: h.store,
		opts:  h.opts,
	}
}

// Create inserts a new entity and records a "create" change.
func (h *HistoryCRUD[T, ID]) Create(ctx context.Context, entity T) (T, error) {
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
func (h *HistoryCRUD[T, ID]) Get(ctx context.Context, id ID, qarg ...r3.Query) (T, error) {
	return h.inner.Get(ctx, id, qarg...)
}

// List retrieves entities. No history is recorded for reads.
func (h *HistoryCRUD[T, ID]) List(ctx context.Context, qarg ...r3.Query) ([]T, int64, error) {
	return h.inner.List(ctx, qarg...)
}

// Update modifies an existing entity and records an "update" change with field-level diff.
func (h *HistoryCRUD[T, ID]) Update(ctx context.Context, entity T) (T, error) {
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
func (h *HistoryCRUD[T, ID]) Patch(ctx context.Context, entity T, fields r3.Fields) (T, error) {
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
func (h *HistoryCRUD[T, ID]) Delete(ctx context.Context, id ID) error {
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
// separately via each rule's SnapshotStore.
func (h *HistoryCRUD[T, ID]) record(ctx context.Context, info recordInfo[T], diffFn func() []FieldChange) {
	buildRecord := func() ChangeRecord {
		recordID := ""
		if h.opts.IDFunc != nil {
			recordID = fmt.Sprint(h.opts.IDFunc(info.entity))
		}

		record := ChangeRecord{
			RecordType: h.opts.RecordType,
			RecordID:   recordID,
			Action:     info.action,
			Changes:    diffFn(),
			CreatedAt:  time.Now(),
		}

		// Metadata
		if h.opts.MetadataFunc != nil {
			record.Metadata = h.opts.MetadataFunc(ctx)
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
			// Get next version (best-effort in async mode)
			if v, err := h.store.NextVersion(context.Background(), record.RecordType, record.RecordID); err == nil {
				record.Version = v
			}
			if err := h.store.Record(context.Background(), record); err != nil {
				log.Printf("r3history: async record failed for %s/%s: %v", record.RecordType, record.RecordID, err)
			}
			// Evaluate snapshot rules
			if len(h.opts.SnapshotRules) > 0 {
				evaluateSnapshotRules(context.Background(), h.opts.SnapshotRules, record.RecordType, record.RecordID, record.Version, info.action, info.old, info.entity, record.Metadata)
			}
		}()
		return
	}

	record := buildRecord()
	// Get next version
	if v, err := h.store.NextVersion(ctx, record.RecordType, record.RecordID); err == nil {
		record.Version = v
	}
	if err := h.store.Record(ctx, record); err != nil {
		log.Printf("r3history: record failed for %s/%s: %v", record.RecordType, record.RecordID, err)
	}

	// Evaluate snapshot rules
	if len(h.opts.SnapshotRules) > 0 {
		evaluateSnapshotRules(ctx, h.opts.SnapshotRules, record.RecordType, record.RecordID, record.Version, info.action, info.old, info.entity, record.Metadata)
	}
}
