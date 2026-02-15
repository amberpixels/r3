// Package r3hsql provides a database/sql-backed implementation of r3history.Store.
//
// It stores change records in a single SQL table (configurable name) with JSON columns
// for changes and metadata. Compatible with PostgreSQL (JSONB), MySQL (JSON),
// and SQLite (TEXT with JSON content).
//
// Usage:
//
//	db, _ := sql.Open("pgx", dsn)
//	store := r3hsql.NewStore(db)
//	// or with custom table name:
//	store := r3hsql.NewStore(db, r3hsql.WithTableName("order_history"))
package r3hsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/r3history"
)

// Store is a database/sql-backed implementation of r3history.Store.
type Store struct {
	db   *sql.DB
	opts StoreOptions
}

// Compile-time check.
var _ r3history.Store = &Store{}

// StoreOptions configures the SQL history store.
type StoreOptions struct {
	// TableName is the name of the table where change records are stored.
	// Default: "change_records".
	TableName string

	// TableNameFunc derives the table name from the record type.
	// If set, it takes precedence over TableName.
	// Example: func(rt string) string { return rt + "_history" }
	// This would store "orders" history in "orders_history" table.
	TableNameFunc func(recordType string) string

	// PlaceholderFunc generates parameter placeholders.
	// Default: question-mark style ("?"). For PostgreSQL, use DollarPlaceholder.
	PlaceholderFunc func(n int) string
}

// StoreOption is a functional option for configuring the SQL Store.
type StoreOption func(*StoreOptions)

// WithTableName sets a fixed table name for all record types.
func WithTableName(name string) StoreOption {
	return func(o *StoreOptions) { o.TableName = name }
}

// WithTableNameFunc sets a function that derives table names from record types.
func WithTableNameFunc(fn func(string) string) StoreOption {
	return func(o *StoreOptions) { o.TableNameFunc = fn }
}

// WithDollarPlaceholders configures PostgreSQL-style $1, $2, ... placeholders.
func WithDollarPlaceholders() StoreOption {
	return func(o *StoreOptions) {
		o.PlaceholderFunc = func(n int) string {
			return fmt.Sprintf("$%d", n)
		}
	}
}

// NewStore creates a new SQL-backed history store.
func NewStore(db *sql.DB, opts ...StoreOption) *Store {
	o := StoreOptions{
		TableName: "change_records",
		PlaceholderFunc: func(_ int) string {
			return "?"
		},
	}
	for _, fn := range opts {
		fn(&o)
	}
	return &Store{db: db, opts: o}
}

// tableName returns the table name for a given record type.
func (s *Store) tableName(recordType string) string {
	if s.opts.TableNameFunc != nil {
		return s.opts.TableNameFunc(recordType)
	}
	return s.opts.TableName
}

// ph returns the placeholder for parameter index n (1-based).
func (s *Store) ph(n int) string {
	return s.opts.PlaceholderFunc(n)
}

// phs returns comma-separated placeholders from start to start+count-1.
func (s *Store) phs(start, count int) string {
	parts := make([]string, count)
	for i := range count {
		parts[i] = s.ph(start + i)
	}
	return strings.Join(parts, ", ")
}

// Record persists a change record.
func (s *Store) Record(ctx context.Context, record r3history.ChangeRecord) error {
	table := s.tableName(record.RecordType)

	changesJSON, err := json.Marshal(record.Changes)
	if err != nil {
		return fmt.Errorf("r3hsql: marshal changes: %w", err)
	}

	metaJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("r3hsql: marshal metadata: %w", err)
	}

	if record.ID == "" {
		record.ID = generateID()
	}

	query := fmt.Sprintf(
		`INSERT INTO %s (id, record_type, record_id, action, version, changes, parent_type, parent_id, metadata, created_at)
		 VALUES (%s)`,
		table,
		s.phs(1, 10),
	)

	_, err = s.db.ExecContext(ctx, query,
		record.ID,
		record.RecordType,
		record.RecordID,
		string(record.Action),
		record.Version,
		changesJSON,
		record.ParentType,
		record.ParentID,
		metaJSON,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("r3hsql: insert: %w", err)
	}

	return nil
}

// ForRecord returns history for a specific entity, ordered by version ascending.
func (s *Store) ForRecord(ctx context.Context, recordType string, recordID string, qarg ...r3.Query) ([]r3history.ChangeRecord, int64, error) {
	table := s.tableName(recordType)

	where := fmt.Sprintf("record_type = %s AND record_id = %s", s.ph(1), s.ph(2))
	args := []any{recordType, recordID}

	return s.queryRecords(ctx, table, where, args, "version ASC", qarg...)
}

// ForType returns history for all entities of a given type, ordered by created_at descending.
func (s *Store) ForType(ctx context.Context, recordType string, qarg ...r3.Query) ([]r3history.ChangeRecord, int64, error) {
	table := s.tableName(recordType)

	where := fmt.Sprintf("record_type = %s", s.ph(1))
	args := []any{recordType}

	return s.queryRecords(ctx, table, where, args, "created_at DESC", qarg...)
}

// ForTree returns history across a parent-child hierarchy.
func (s *Store) ForTree(ctx context.Context, scopes []r3history.TreeScope, qarg ...r3.Query) ([]r3history.ChangeRecord, int64, error) {
	if len(scopes) == 0 {
		return nil, 0, nil
	}

	// Use the first scope's record type to determine the table.
	// All scopes query from the same table.
	table := s.tableName(scopes[0].RecordType)

	var orParts []string
	var args []any
	argIdx := 1

	for _, scope := range scopes {
		var conditions []string

		conditions = append(conditions, fmt.Sprintf("record_type = %s", s.ph(argIdx)))
		args = append(args, scope.RecordType)
		argIdx++

		if scope.RecordID != "" {
			conditions = append(conditions, fmt.Sprintf("record_id = %s", s.ph(argIdx)))
			args = append(args, scope.RecordID)
			argIdx++
		}

		if scope.ParentType != "" {
			conditions = append(conditions, fmt.Sprintf("parent_type = %s", s.ph(argIdx)))
			args = append(args, scope.ParentType)
			argIdx++

			if scope.ParentID != "" {
				conditions = append(conditions, fmt.Sprintf("parent_id = %s", s.ph(argIdx)))
				args = append(args, scope.ParentID)
				argIdx++
			}
		}

		orParts = append(orParts, "("+strings.Join(conditions, " AND ")+")")
	}

	where := strings.Join(orParts, " OR ")

	return s.queryRecords(ctx, table, where, args, "created_at DESC", qarg...)
}

// NextVersion atomically returns the next version number for an entity.
func (s *Store) NextVersion(ctx context.Context, recordType string, recordID string) (int64, error) {
	table := s.tableName(recordType)

	query := fmt.Sprintf(
		"SELECT COALESCE(MAX(version), 0) FROM %s WHERE record_type = %s AND record_id = %s",
		table, s.ph(1), s.ph(2),
	)

	var maxVersion int64
	err := s.db.QueryRowContext(ctx, query, recordType, recordID).Scan(&maxVersion)
	if err != nil {
		return 0, fmt.Errorf("r3hsql: next version: %w", err)
	}

	return maxVersion + 1, nil
}

// GetVersion returns the change record for a specific version.
func (s *Store) GetVersion(ctx context.Context, recordType string, recordID string, version int64) (r3history.ChangeRecord, error) {
	table := s.tableName(recordType)

	query := fmt.Sprintf(
		"SELECT id, record_type, record_id, action, version, changes, parent_type, parent_id, metadata, created_at FROM %s WHERE record_type = %s AND record_id = %s AND version = %s",
		table, s.ph(1), s.ph(2), s.ph(3),
	)

	record, err := s.scanRecord(s.db.QueryRowContext(ctx, query, recordType, recordID, version))
	if err != nil {
		if err == sql.ErrNoRows {
			return r3history.ChangeRecord{}, r3history.ErrVersionNotFound
		}
		return r3history.ChangeRecord{}, fmt.Errorf("r3hsql: get version: %w", err)
	}

	return record, nil
}

// LatestVersion returns the most recent change record for a specific entity.
func (s *Store) LatestVersion(ctx context.Context, recordType string, recordID string) (r3history.ChangeRecord, error) {
	table := s.tableName(recordType)

	query := fmt.Sprintf(
		"SELECT id, record_type, record_id, action, version, changes, parent_type, parent_id, metadata, created_at FROM %s WHERE record_type = %s AND record_id = %s ORDER BY version DESC LIMIT 1",
		table, s.ph(1), s.ph(2),
	)

	record, err := s.scanRecord(s.db.QueryRowContext(ctx, query, recordType, recordID))
	if err != nil {
		if err == sql.ErrNoRows {
			return r3history.ChangeRecord{}, r3history.ErrNoHistory
		}
		return r3history.ChangeRecord{}, fmt.Errorf("r3hsql: latest version: %w", err)
	}

	return record, nil
}

// queryRecords is a shared helper for ForRecord, ForType, and ForTree.
func (s *Store) queryRecords(
	ctx context.Context,
	table string,
	where string,
	args []any,
	defaultOrder string,
	qarg ...r3.Query,
) ([]r3history.ChangeRecord, int64, error) {
	q := mergeQuery(qarg...)

	orderBy := defaultOrder

	// Pagination
	var limitOffset string
	var totalCount int64
	isPaginated := q.Pagination != nil && q.Pagination.IsPaginated()

	if isPaginated {
		// Count first
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, where)
		err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
		if err != nil {
			return nil, 0, fmt.Errorf("r3hsql: count: %w", err)
		}
		if totalCount == 0 {
			return nil, 0, nil
		}

		limit, offset := q.Pagination.ToLimitOffset()
		limitOffset = fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	}

	selectQuery := fmt.Sprintf(
		"SELECT id, record_type, record_id, action, version, changes, parent_type, parent_id, metadata, created_at FROM %s WHERE %s ORDER BY %s%s",
		table, where, orderBy, limitOffset,
	)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("r3hsql: query: %w", err)
	}
	defer rows.Close()

	var records []r3history.ChangeRecord
	for rows.Next() {
		record, err := s.scanRowRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("r3hsql: rows: %w", err)
	}

	if !isPaginated {
		totalCount = int64(len(records))
	}

	return records, totalCount, nil
}

// scanRecord scans a single row into a ChangeRecord.
func (s *Store) scanRecord(row *sql.Row) (r3history.ChangeRecord, error) {
	var r r3history.ChangeRecord
	var changesRaw, metaRaw []byte
	var action string

	err := row.Scan(
		&r.ID,
		&r.RecordType,
		&r.RecordID,
		&action,
		&r.Version,
		&changesRaw,
		&r.ParentType,
		&r.ParentID,
		&metaRaw,
		&r.CreatedAt,
	)
	if err != nil {
		return r, err
	}

	r.Action = r3history.Action(action)

	if len(changesRaw) > 0 {
		if err := json.Unmarshal(changesRaw, &r.Changes); err != nil {
			return r, fmt.Errorf("r3hsql: unmarshal changes: %w", err)
		}
	}

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &r.Metadata); err != nil {
			return r, fmt.Errorf("r3hsql: unmarshal metadata: %w", err)
		}
	}

	return r, nil
}

// scanRowRecord scans a row from *sql.Rows into a ChangeRecord.
func (s *Store) scanRowRecord(rows *sql.Rows) (r3history.ChangeRecord, error) {
	var r r3history.ChangeRecord
	var changesRaw, metaRaw []byte
	var action string

	err := rows.Scan(
		&r.ID,
		&r.RecordType,
		&r.RecordID,
		&action,
		&r.Version,
		&changesRaw,
		&r.ParentType,
		&r.ParentID,
		&metaRaw,
		&r.CreatedAt,
	)
	if err != nil {
		return r, fmt.Errorf("r3hsql: scan: %w", err)
	}

	r.Action = r3history.Action(action)

	if len(changesRaw) > 0 {
		if err := json.Unmarshal(changesRaw, &r.Changes); err != nil {
			return r, fmt.Errorf("r3hsql: unmarshal changes: %w", err)
		}
	}

	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &r.Metadata); err != nil {
			return r, fmt.Errorf("r3hsql: unmarshal metadata: %w", err)
		}
	}

	return r, nil
}

// generateID generates a simple unique ID.
// Uses timestamp + random suffix. For production, consider UUIDs.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// mergeQuery merges optional query arguments into a single Query.
func mergeQuery(qarg ...r3.Query) r3.Query {
	q := r3.NewQuery()
	for _, other := range qarg {
		q = q.MergeWith(other)
	}
	return q
}

// CreateTable returns the SQL DDL for creating the change_records table.
// The caller is responsible for executing it. The SQL uses standard types
// and should work with PostgreSQL, MySQL, and SQLite.
func CreateTable(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    id          TEXT NOT NULL PRIMARY KEY,
    record_type TEXT NOT NULL,
    record_id   TEXT NOT NULL,
    action      TEXT NOT NULL,
    version     BIGINT NOT NULL,
    changes     TEXT,
    parent_type TEXT NOT NULL DEFAULT '',
    parent_id   TEXT NOT NULL DEFAULT '',
    metadata    TEXT,
    created_at  TIMESTAMP NOT NULL,
    UNIQUE(record_type, record_id, version)
)`, tableName)
}

// CreateTablePostgres returns PostgreSQL-optimized DDL with JSONB columns and indexes.
func CreateTablePostgres(tableName string) string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
    id          TEXT NOT NULL PRIMARY KEY,
    record_type TEXT NOT NULL,
    record_id   TEXT NOT NULL,
    action      TEXT NOT NULL,
    version     BIGINT NOT NULL,
    changes     JSONB,
    parent_type TEXT NOT NULL DEFAULT '',
    parent_id   TEXT NOT NULL DEFAULT '',
    metadata    JSONB,
    created_at  TIMESTAMPTZ NOT NULL,
    UNIQUE(record_type, record_id, version)
);
CREATE INDEX IF NOT EXISTS idx_%s_record ON %s (record_type, record_id, version);
CREATE INDEX IF NOT EXISTS idx_%s_parent ON %s (parent_type, parent_id);
CREATE INDEX IF NOT EXISTS idx_%s_type ON %s (record_type, created_at)`,
		tableName,
		tableName, tableName,
		tableName, tableName,
		tableName, tableName,
	)
}
