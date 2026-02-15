package r3history

import (
	"context"
	"errors"

	"github.com/amberpixels/r3"
)

// Sentinel errors for Store operations.
var (
	// ErrVersionNotFound is returned when a requested version does not exist.
	ErrVersionNotFound = errors.New("r3history: version not found")

	// ErrRecordNotFound is returned when a requested change record does not exist.
	ErrRecordNotFound = errors.New("r3history: record not found")

	// ErrNoHistory is returned when attempting to revert an entity with no history.
	ErrNoHistory = errors.New("r3history: no history for this record")
)

// Store is the backend-agnostic interface for persisting and querying change records.
//
// Implementations exist for different storage systems (SQL, GORM, MongoDB, etc.),
// allowing history to be stored independently from the entity data. For example,
// entities may live in PostgreSQL while their history is stored in MongoDB.
//
// All query methods accept optional r3.Query arguments for filtering, sorting,
// and pagination — reusing the same query primitives as the main CRUD interface.
type Store interface {
	// Record persists a single change record.
	// The store is responsible for serializing Changes and Metadata if needed.
	Record(ctx context.Context, record ChangeRecord) error

	// ForRecord returns the change history of a specific entity instance,
	// ordered by version (ascending by default).
	// Example: all changes to Order #42.
	ForRecord(ctx context.Context, recordType string, recordID string, qarg ...r3.Query) ([]ChangeRecord, int64, error)

	// ForType returns the change history for all entities of a given type,
	// ordered by created_at (descending by default).
	// Example: all order changes across all orders.
	ForType(ctx context.Context, recordType string, qarg ...r3.Query) ([]ChangeRecord, int64, error)

	// ForTree returns change history across a parent-child hierarchy.
	// This enables queries like "all changes to Campaign #5 AND its adsets AND their creatives".
	// Each TreeScope defines one level of the hierarchy.
	ForTree(ctx context.Context, scopes []TreeScope, qarg ...r3.Query) ([]ChangeRecord, int64, error)

	// NextVersion atomically returns the next version number for a (recordType, recordID) pair.
	// Version 1 is the first version (the create). Implementations must ensure
	// sequential, gap-free version numbers per entity.
	NextVersion(ctx context.Context, recordType string, recordID string) (int64, error)

	// GetVersion returns the change record for a specific version of a specific entity.
	// Returns ErrVersionNotFound if the version does not exist.
	GetVersion(ctx context.Context, recordType string, recordID string, version int64) (ChangeRecord, error)

	// LatestVersion returns the most recent change record for a specific entity.
	// Returns ErrNoHistory if the entity has no history.
	LatestVersion(ctx context.Context, recordType string, recordID string) (ChangeRecord, error)
}
