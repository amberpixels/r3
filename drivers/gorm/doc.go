// Package r3gorm provides an r3.CRUD[T, ID] driver backed by GORM.
//
// Driver: gorm.io/gorm
// Source: https://gorm.io
//
// Supported r3 features:
//   - Full CRUD (Create, Get, List, Update, Patch, Delete)
//   - Filters, Sorts, Pagination via the r3 SQL dialect
//   - Preloads via GORM's Preload()
//   - Soft-delete via GORM's Unscoped()
//   - Transactions via the r3.Transactor interface
//   - Thread-safe default queries (SetDefaultListQuery, SetDefaultGetQuery)
//   - Raw escape hatch (GormRaw) for custom gorm.DB usage
package r3gorm
