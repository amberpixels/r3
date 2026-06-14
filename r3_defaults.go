package r3

import (
	"sync"
)

// Defaults stores the default query values for List and Get operations.
// It is shared by all drivers (database/sql-based, GORM, Bun, go-pg, MongoDB, etc.).
type Defaults struct {
	ListQuery Query
	GetQuery  Query
}

// NewDefaults returns Defaults initialized with reasonable default queries.
func NewDefaults() Defaults {
	return Defaults{
		ListQuery: DefaultQuery(),
		GetQuery:  DefaultQuery(),
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

// NewDefaultsManagerWithConfig creates a DefaultsManager that respects the given Config.
//
// If Config.Defaults.Unpaginated is set, the default list query is unbounded
// (List returns all rows unless a query opts into pagination). Otherwise, if
// Config.Defaults.PageSize differs from the global default, the default list
// query is initialized with that page size.
func NewDefaultsManagerWithConfig(cfg Config) DefaultsManager {
	d := NewDefaults()
	switch {
	case cfg.Defaults.Unpaginated:
		q := DefaultQuery()
		q.Pagination = NoPagination()
		d.ListQuery = q
	case cfg.Defaults.PageSize > 0 && cfg.Defaults.PageSize != PageSizeDefault:
		q := DefaultQuery()
		q.Pagination = NewPaginationSpecWithSize(cfg.Defaults.PageSize)
		d.ListQuery = q
	}
	return DefaultsManager{
		defaults: d,
	}
}

// SetDefaultListQuery sets the default ListQuery (thread-safe).
func (dm *DefaultsManager) SetDefaultListQuery(q Query) {
	dm.mu.Lock()
	dm.defaults.ListQuery = q
	dm.mu.Unlock()
}

// SetDefaultGetQuery sets the default GetQuery (thread-safe).
func (dm *DefaultsManager) SetDefaultGetQuery(q Query) {
	dm.mu.Lock()
	dm.defaults.GetQuery = q
	dm.mu.Unlock()
}

// GetDefaultListQuery returns the default ListQuery (thread-safe).
func (dm *DefaultsManager) GetDefaultListQuery() Query {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.defaults.ListQuery
}

// GetDefaultGetQuery returns the default GetQuery (thread-safe).
func (dm *DefaultsManager) GetDefaultGetQuery() Query {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.defaults.GetQuery
}

// MergeListQuery merges the given query args with the default list query.
func (dm *DefaultsManager) MergeListQuery(qarg ...Query) Query {
	q := dm.GetDefaultListQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}
	return q
}

// MergeGetQuery merges the given query args with the default get query.
func (dm *DefaultsManager) MergeGetQuery(qarg ...Query) Query {
	q := dm.GetDefaultGetQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}
	return q
}
