package history

import (
	"context"

	"github.com/amberpixels/r3"
	r3utils "github.com/amberpixels/r3/internal/utils"
)

// MetadataFunc extracts surrounding-context Metadata (Source, Extra) from a
// context. The actor is NOT set here - it is a first-class field resolved from
// r3.GetActor(ctx).
//
//	func metadataFromCtx(ctx context.Context) Metadata {
//	    return r3history.Metadata{
//	        Source: "admin_ui",
//	        Extra:  map[string]string{"request_id": reqID(ctx)},
//	    }
//	}
type MetadataFunc func(ctx context.Context) Metadata

// DiffFunc computes field-level changes between two entities, replacing the
// default reflection-based differ.
type DiffFunc[T any] func(old, cur T) []FieldChange

// IDFunc extracts an entity's primary key.
type IDFunc[T any, ID comparable] func(entity T) ID

// ParentRef links a child entity to its parent so every change record carries the
// parent's type and ID, enabling ForTree() queries.
type ParentRef struct {
	// ParentType is the parent entity's RecordType (e.g. "campaigns").
	ParentType string

	// FKField is the child's Go struct field holding the parent ID (e.g.
	// "CampaignID"), read via reflection to populate ParentID.
	FKField string
}

// Options configures a CRUD decorator.
type Options[T any, ID comparable] struct {
	// RecordType identifies this entity type in change records. If empty, derived
	// from T (e.g. Order -> "orders", CampaignAdset -> "campaign_adsets").
	RecordType string

	// MetadataFunc extracts surrounding-context metadata; it does not affect the
	// actor, which is a first-class field resolved from r3.GetActor(ctx).
	MetadataFunc MetadataFunc

	// FixedActor, when set, attributes every change to this actor instead of
	// r3.GetActor(ctx). Use for a system/worker repo whose background contexts
	// carry no principal. Nil resolves the actor per-call from context.
	FixedActor *r3.Actor

	// DiffFunc overrides the default reflection-based Diff[T].
	DiffFunc DiffFunc[T]

	// IDFunc extracts the primary key. Required - needed to fetch old state before
	// updates.
	IDFunc IDFunc[T, ID]

	// ParentRef links this entity to a parent for tree queries; nil omits parent
	// info.
	ParentRef *ParentRef

	// SnapshotRules gate opt-in full-entity snapshots, stored separately from
	// change records with their own trigger conditions. See [SnapshotRule].
	SnapshotRules []SnapshotRule[T]

	// Async records history in a background goroutine; the CRUD op returns
	// immediately and recording errors go to ErrorHandler only (FailOnError is
	// ignored). Default false.
	Async bool

	// ErrorHandler is invoked on a failed change-record write, overriding the
	// default slog logging. It does not control whether the op fails - see
	// FailOnError.
	ErrorHandler func(error)

	// FailOnError (sync mode) returns a failed change-record write's error from the
	// CRUD op, surfacing audit gaps.
	//
	// NOTE: history is recorded AFTER the mutation succeeds, so when this fires the
	// change has already been applied; the error signals it could not be audited,
	// not that the mutation rolled back. For atomic audit, use a transaction.
	// Ignored in async mode. Default false.
	FailOnError bool
}

// Option is a functional option for configuring CRUD.
type Option[T any, ID comparable] func(*Options[T, ID])

// WithRecordType sets an explicit record type name.
func WithRecordType[T any, ID comparable](name string) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.RecordType = name
	}
}

// WithMetadataFunc sets the function that extracts metadata from context.
func WithMetadataFunc[T any, ID comparable](fn MetadataFunc) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.MetadataFunc = fn
	}
}

// WithFixedActor attributes every change to actor, ignoring the context actor.
// See [Options.FixedActor].
func WithFixedActor[T any, ID comparable](actor r3.Actor) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.FixedActor = &actor
	}
}

// WithDiffFunc sets a custom diff function.
func WithDiffFunc[T any, ID comparable](fn DiffFunc[T]) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.DiffFunc = fn
	}
}

// WithIDFunc sets the function that extracts the primary key from an entity.
func WithIDFunc[T any, ID comparable](fn IDFunc[T, ID]) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.IDFunc = fn
	}
}

// WithParentRef configures parent-child linking for tree queries.
func WithParentRef[T any, ID comparable](parentType string, fkField string) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.ParentRef = &ParentRef{
			ParentType: parentType,
			FKField:    fkField,
		}
	}
}

// WithSnapshotRules configures opt-in snapshot rules (stored separately from
// change records).
func WithSnapshotRules[T any, ID comparable](rules ...SnapshotRule[T]) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.SnapshotRules = append(o.SnapshotRules, rules...)
	}
}

// WithAsync enables asynchronous history recording.
func WithAsync[T any, ID comparable]() Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.Async = true
	}
}

// WithErrorHandler sets a handler for a failed change-record write, overriding
// default slog logging. It does not change whether the op fails - use
// WithFailOnError.
func WithErrorHandler[T any, ID comparable](fn func(error)) Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.ErrorHandler = fn
	}
}

// WithFailOnError returns a failed change-record write's error from the CRUD op
// (sync mode). See [Options.FailOnError] for the caveat that the mutation has
// already been applied. Ignored in async mode.
func WithFailOnError[T any, ID comparable]() Option[T, ID] {
	return func(o *Options[T, ID]) {
		o.FailOnError = true
	}
}

// applyDefaults fills in zero-value options with sensible defaults.
func applyDefaults[T any, ID comparable](opts *Options[T, ID]) {
	if opts.RecordType == "" {
		opts.RecordType = deriveRecordType[T]()
	}
	if opts.DiffFunc == nil {
		opts.DiffFunc = Diff[T]
	}
}

// deriveRecordType is the struct name of T as snake_case plural (Order -> "orders").
func deriveRecordType[T any]() string {
	return r3utils.ToSnakeCasePlural(typeName[T]())
}
