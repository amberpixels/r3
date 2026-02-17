package history

import (
	"context"

	r3utils "github.com/amberpixels/r3/internal/utils"
)

// MetadataFunc extracts Metadata from a context.Context.
// Typically this reads actor info from middleware-injected context values.
//
// Example:
//
//	func metadataFromCtx(ctx context.Context) Metadata {
//	    user := auth.UserFromContext(ctx)
//	    return r3history.Metadata{
//	        ActorID:   user.ID,
//	        ActorType: "user",
//	        Source:    "api",
//	    }
//	}
type MetadataFunc func(ctx context.Context) Metadata

// DiffFunc computes field-level changes between two entities.
// Use this to provide a custom diff implementation instead of the default
// reflection-based differ.
type DiffFunc[T any] func(old, cur T) []FieldChange

// IDFunc extracts the primary key from an entity and returns it as a string.
// Used by the decorator to obtain the record ID for change records.
type IDFunc[T any, ID comparable] func(entity T) ID

// ParentRef links a child entity to its parent for tree/nested history queries.
// When configured, every change record for this entity will include the parent's
// type and ID, enabling ForTree() queries.
type ParentRef struct {
	// ParentType is the RecordType of the parent entity (e.g. "campaigns").
	ParentType string

	// FKField is the Go struct field name on the child that holds the parent's ID
	// (e.g. "CampaignID"). The decorator reads this field via reflection to populate
	// ParentID on each change record.
	FKField string
}

// Options configures the behavior of a CRUD decorator.
type Options[T any, ID comparable] struct {
	// RecordType is the name used to identify this entity type in change records.
	// If empty, it is derived automatically from the struct type T
	// (e.g. Order -> "orders", CampaignAdset -> "campaign_adsets").
	RecordType string

	// MetadataFunc extracts actor/context metadata from the request context.
	// If nil, the history decorator still populates ActorID and ActorType
	// from r3.GetActor(ctx) automatically. If MetadataFunc is set but
	// leaves ActorID/ActorType empty, those are filled from the Actor context.
	MetadataFunc MetadataFunc

	// DiffFunc provides a custom diff implementation.
	// If nil, the default reflection-based Diff[T] is used.
	DiffFunc DiffFunc[T]

	// IDFunc extracts the primary key from an entity.
	// This is required — the decorator needs to know how to get the ID
	// from an entity for fetching the old state before updates.
	IDFunc IDFunc[T, ID]

	// ParentRef links this entity to a parent for tree queries.
	// If nil, change records will not include parent information.
	ParentRef *ParentRef

	// SnapshotRules defines conditions under which full entity snapshots
	// are taken and stored in a separate SnapshotStore. Snapshots are
	// completely decoupled from change records — they have their own
	// storage (separate table/collection) and custom trigger conditions.
	//
	// Example: snapshot a BenefitSheet when status changes from "draft" to "published".
	SnapshotRules []SnapshotRule[T]

	// Async when true records history in a background goroutine.
	// The CRUD operation returns immediately; history recording errors are logged
	// but do not affect the CRUD result. Default: false (synchronous).
	Async bool
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

// WithSnapshotRules configures snapshot rules for opt-in snapshotting.
// Snapshots are stored separately from change records via a SnapshotStore.
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

// applyDefaults fills in zero-value options with sensible defaults.
func applyDefaults[T any, ID comparable](opts *Options[T, ID]) {
	if opts.RecordType == "" {
		opts.RecordType = deriveRecordType[T]()
	}
	if opts.DiffFunc == nil {
		opts.DiffFunc = Diff[T]
	}
}

// deriveRecordType derives the record type name from the struct type T.
// It converts the struct name to snake_case plural (e.g. Order -> "orders").
func deriveRecordType[T any]() string {
	return r3utils.ToSnakeCasePlural(typeName[T]())
}
