// Package r3hgorm provides a GORM-backed implementation of r3history.Store.
//
// It stores change records using GORM's standard model conventions.
// The store can use the same *gorm.DB as your entity CRUD or a completely
// different database connection.
//
// Usage:
//
//	store := r3hgorm.NewStore(gormDB)
//	// optionally auto-migrate the table:
//	store.AutoMigrate()
package r3hgorm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/r3history"
	"gorm.io/gorm"
)

// ChangeRecordModel is the GORM model for the change_records table.
// It mirrors r3history.ChangeRecord but uses GORM-compatible types.
type ChangeRecordModel struct {
	ID         string    `gorm:"column:id;primaryKey"`
	RecordType string    `gorm:"column:record_type;index:idx_cr_record;index:idx_cr_type;not null"`
	RecordID   string    `gorm:"column:record_id;index:idx_cr_record;not null"`
	Action     string    `gorm:"column:action;not null"`
	Version    int64     `gorm:"column:version;index:idx_cr_record;not null"`
	Changes    string    `gorm:"column:changes;type:text"`
	ParentType string    `gorm:"column:parent_type;index:idx_cr_parent;not null;default:''"`
	ParentID   string    `gorm:"column:parent_id;index:idx_cr_parent;not null;default:''"`
	Metadata   string    `gorm:"column:metadata;type:text"`
	CreatedAt  time.Time `gorm:"column:created_at;index:idx_cr_type;not null"`
}

// TableName returns the table name for GORM.
func (m ChangeRecordModel) TableName() string {
	return "change_records"
}

// Store is a GORM-backed implementation of r3history.Store.
type Store struct {
	db   *gorm.DB
	opts StoreOptions
}

// Compile-time check.
var _ r3history.Store = &Store{}

// StoreOptions configures the GORM history store.
type StoreOptions struct {
	// TableName overrides the default table name ("change_records").
	TableName string

	// TableNameFunc derives the table name from the record type.
	// Takes precedence over TableName if set.
	TableNameFunc func(recordType string) string
}

// StoreOption is a functional option for configuring the Store.
type StoreOption func(*StoreOptions)

// WithTableName sets a fixed table name.
func WithTableName(name string) StoreOption {
	return func(o *StoreOptions) { o.TableName = name }
}

// WithTableNameFunc sets a function that derives table names from record types.
func WithTableNameFunc(fn func(string) string) StoreOption {
	return func(o *StoreOptions) { o.TableNameFunc = fn }
}

// NewStore creates a new GORM-backed history store.
func NewStore(db *gorm.DB, opts ...StoreOption) *Store {
	o := StoreOptions{
		TableName: "change_records",
	}
	for _, fn := range opts {
		fn(&o)
	}
	return &Store{db: db, opts: o}
}

// AutoMigrate creates or updates the change records table schema.
func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(&ChangeRecordModel{})
}

// tableName returns the table name for a given record type.
func (s *Store) tableName(recordType string) string {
	if s.opts.TableNameFunc != nil {
		return s.opts.TableNameFunc(recordType)
	}
	return s.opts.TableName
}

// Record persists a change record.
func (s *Store) Record(ctx context.Context, record r3history.ChangeRecord) error {
	changesJSON, err := json.Marshal(record.Changes)
	if err != nil {
		return fmt.Errorf("r3hgorm: marshal changes: %w", err)
	}

	metaJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("r3hgorm: marshal metadata: %w", err)
	}

	if record.ID == "" {
		record.ID = generateID()
	}

	model := ChangeRecordModel{
		ID:         record.ID,
		RecordType: record.RecordType,
		RecordID:   record.RecordID,
		Action:     string(record.Action),
		Version:    record.Version,
		Changes:    string(changesJSON),
		ParentType: record.ParentType,
		ParentID:   record.ParentID,
		Metadata:   string(metaJSON),
		CreatedAt:  record.CreatedAt,
	}

	table := s.tableName(record.RecordType)
	result := s.db.WithContext(ctx).Table(table).Create(&model)
	if result.Error != nil {
		return fmt.Errorf("r3hgorm: insert: %w", result.Error)
	}

	return nil
}

// ForRecord returns history for a specific entity, ordered by version ascending.
func (s *Store) ForRecord(ctx context.Context, recordType string, recordID string, qarg ...r3.Query) ([]r3history.ChangeRecord, int64, error) {
	table := s.tableName(recordType)
	query := s.db.WithContext(ctx).Table(table).
		Where("record_type = ? AND record_id = ?", recordType, recordID).
		Order("version ASC")

	return s.queryRecords(query, qarg...)
}

// ForType returns history for all entities of a given type, ordered by created_at descending.
func (s *Store) ForType(ctx context.Context, recordType string, qarg ...r3.Query) ([]r3history.ChangeRecord, int64, error) {
	table := s.tableName(recordType)
	query := s.db.WithContext(ctx).Table(table).
		Where("record_type = ?", recordType).
		Order("created_at DESC")

	return s.queryRecords(query, qarg...)
}

// ForTree returns history across a parent-child hierarchy.
func (s *Store) ForTree(ctx context.Context, scopes []r3history.TreeScope, qarg ...r3.Query) ([]r3history.ChangeRecord, int64, error) {
	if len(scopes) == 0 {
		return nil, 0, nil
	}

	table := s.tableName(scopes[0].RecordType)
	query := s.db.WithContext(ctx).Table(table)

	// Build OR conditions for each scope
	combined := s.db.WithContext(ctx)
	for i, scope := range scopes {
		scopeQ := s.db.Where("record_type = ?", scope.RecordType)

		if scope.RecordID != "" {
			scopeQ = scopeQ.Where("record_id = ?", scope.RecordID)
		}
		if scope.ParentType != "" {
			scopeQ = scopeQ.Where("parent_type = ?", scope.ParentType)
			if scope.ParentID != "" {
				scopeQ = scopeQ.Where("parent_id = ?", scope.ParentID)
			}
		}

		if i == 0 {
			combined = scopeQ
		} else {
			combined = combined.Or(scopeQ)
		}
	}

	query = query.Where(combined).Order("created_at DESC")

	return s.queryRecords(query, qarg...)
}

// NextVersion atomically returns the next version number.
func (s *Store) NextVersion(ctx context.Context, recordType string, recordID string) (int64, error) {
	table := s.tableName(recordType)

	var maxVersion *int64
	result := s.db.WithContext(ctx).Table(table).
		Where("record_type = ? AND record_id = ?", recordType, recordID).
		Select("MAX(version)").
		Scan(&maxVersion)

	if result.Error != nil {
		return 0, fmt.Errorf("r3hgorm: next version: %w", result.Error)
	}

	if maxVersion == nil {
		return 1, nil
	}
	return *maxVersion + 1, nil
}

// GetVersion returns the change record for a specific version.
func (s *Store) GetVersion(ctx context.Context, recordType string, recordID string, version int64) (r3history.ChangeRecord, error) {
	table := s.tableName(recordType)

	var model ChangeRecordModel
	result := s.db.WithContext(ctx).Table(table).
		Where("record_type = ? AND record_id = ? AND version = ?", recordType, recordID, version).
		First(&model)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return r3history.ChangeRecord{}, r3history.ErrVersionNotFound
		}
		return r3history.ChangeRecord{}, fmt.Errorf("r3hgorm: get version: %w", result.Error)
	}

	return modelToRecord(model)
}

// LatestVersion returns the most recent change record.
func (s *Store) LatestVersion(ctx context.Context, recordType string, recordID string) (r3history.ChangeRecord, error) {
	table := s.tableName(recordType)

	var model ChangeRecordModel
	result := s.db.WithContext(ctx).Table(table).
		Where("record_type = ? AND record_id = ?", recordType, recordID).
		Order("version DESC").
		First(&model)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return r3history.ChangeRecord{}, r3history.ErrNoHistory
		}
		return r3history.ChangeRecord{}, fmt.Errorf("r3hgorm: latest version: %w", result.Error)
	}

	return modelToRecord(model)
}

// queryRecords is a shared helper for executing paginated queries.
func (s *Store) queryRecords(query *gorm.DB, qarg ...r3.Query) ([]r3history.ChangeRecord, int64, error) {
	q := mergeQuery(qarg...)

	isPaginated := q.Pagination != nil && q.Pagination.IsPaginated()

	var totalCount int64
	if isPaginated {
		if err := query.Count(&totalCount).Error; err != nil {
			return nil, 0, fmt.Errorf("r3hgorm: count: %w", err)
		}
		if totalCount == 0 {
			return nil, 0, nil
		}

		limit, offset := q.Pagination.ToLimitOffset()
		query = query.Limit(limit).Offset(offset)
	}

	var models []ChangeRecordModel
	if err := query.Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("r3hgorm: query: %w", err)
	}

	records := make([]r3history.ChangeRecord, 0, len(models))
	for _, m := range models {
		rec, err := modelToRecord(m)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, rec)
	}

	if !isPaginated {
		totalCount = int64(len(records))
	}

	return records, totalCount, nil
}

// modelToRecord converts a GORM model to an r3history.ChangeRecord.
func modelToRecord(m ChangeRecordModel) (r3history.ChangeRecord, error) {
	var record r3history.ChangeRecord

	record.ID = m.ID
	record.RecordType = m.RecordType
	record.RecordID = m.RecordID
	record.Action = r3history.Action(m.Action)
	record.Version = m.Version
	record.ParentType = m.ParentType
	record.ParentID = m.ParentID
	record.CreatedAt = m.CreatedAt

	if m.Changes != "" {
		if err := json.Unmarshal([]byte(m.Changes), &record.Changes); err != nil {
			return record, fmt.Errorf("r3hgorm: unmarshal changes: %w", err)
		}
	}

	if m.Metadata != "" {
		if err := json.Unmarshal([]byte(m.Metadata), &record.Metadata); err != nil {
			return record, fmt.Errorf("r3hgorm: unmarshal metadata: %w", err)
		}
	}

	return record, nil
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func mergeQuery(qarg ...r3.Query) r3.Query {
	q := r3.NewQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}
	return q
}
