package sqlbase

import (
	"sync"

	"github.com/amberpixels/r3"
)

// Defaults stores the default query values for List and Get operations.
// It is shared by all drivers (database/sql-based, GORM, Bun, go-pg, etc.).
type Defaults struct {
	ListQuery r3.Query
	GetQuery  r3.Query
}

// NewDefaults returns Defaults initialized with reasonable default queries.
func NewDefaults() Defaults {
	return Defaults{
		ListQuery: r3.DefaultQuery(),
		GetQuery:  r3.DefaultQuery(),
	}
}

// DefaultsManager provides thread-safe access to Defaults.
// Embed this in your CRUD struct to get SetDefaultListQuery, SetDefaultGetQuery,
// GetDefaultListQuery, and GetDefaultGetQuery for free.
type DefaultsManager struct {
	defaults Defaults
	mu       sync.RWMutex
}

// NewDefaultsManager creates a DefaultsManager with reasonable defaults.
func NewDefaultsManager() DefaultsManager {
	return DefaultsManager{
		defaults: NewDefaults(),
	}
}

// SetDefaultListQuery sets the default ListQuery (thread-safe).
func (dm *DefaultsManager) SetDefaultListQuery(q r3.Query) {
	dm.mu.Lock()
	dm.defaults.ListQuery = q
	dm.mu.Unlock()
}

// SetDefaultGetQuery sets the default GetQuery (thread-safe).
func (dm *DefaultsManager) SetDefaultGetQuery(q r3.Query) {
	dm.mu.Lock()
	dm.defaults.GetQuery = q
	dm.mu.Unlock()
}

// GetDefaultListQuery returns the default ListQuery (thread-safe).
func (dm *DefaultsManager) GetDefaultListQuery() r3.Query {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.defaults.ListQuery
}

// GetDefaultGetQuery returns the default GetQuery (thread-safe).
func (dm *DefaultsManager) GetDefaultGetQuery() r3.Query {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.defaults.GetQuery
}

// MergeListQuery merges the given query args with the default list query.
func (dm *DefaultsManager) MergeListQuery(qarg ...r3.Query) r3.Query {
	q := dm.GetDefaultListQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}
	return q
}

// MergeGetQuery merges the given query args with the default get query.
func (dm *DefaultsManager) MergeGetQuery(qarg ...r3.Query) r3.Query {
	q := dm.GetDefaultGetQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}
	return q
}
