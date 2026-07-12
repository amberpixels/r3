package r3

import (
	"sync"
)

// Defaults holds the default queries for List and Get, shared across all drivers.
type Defaults struct {
	ListQuery Query
	GetQuery  Query
}

// NewDefaults returns Defaults with reasonable default queries.
func NewDefaults() Defaults {
	return Defaults{
		ListQuery: DefaultQuery(),
		GetQuery:  DefaultQuery(),
	}
}

// DefaultsManager is thread-safe access to Defaults; embed it in a CRUD struct to
// get the Set/Get default-query methods for free.
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

// NewDefaultsManagerWithConfig creates a DefaultsManager honoring cfg: Unpaginated
// makes the default list query unbounded; otherwise a non-default PageSize seeds
// the default list query's page size.
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
