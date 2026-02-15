// Package r3hmongo provides a MongoDB-backed implementation of history.Store.
//
// It stores change records in a MongoDB collection. The store can use the same
// database as your entity CRUD or a completely different MongoDB instance.
//
// Usage:
//
//	coll := mongoClient.Database("mydb").Collection("change_records")
//	store := r3hmongo.NewStore(coll)
package r3hmongo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/history"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// changeRecordDoc is the BSON document representation of a ChangeRecord.
type changeRecordDoc struct {
	ID         string    `bson:"_id"`
	RecordType string    `bson:"record_type"`
	RecordID   string    `bson:"record_id"`
	Action     string    `bson:"action"`
	Version    int64     `bson:"version"`
	Changes    string    `bson:"changes"` // JSON string
	ParentType string    `bson:"parent_type,omitempty"`
	ParentID   string    `bson:"parent_id,omitempty"`
	Metadata   string    `bson:"metadata"` // JSON string
	CreatedAt  time.Time `bson:"created_at"`
}

// Store is a MongoDB-backed implementation of history.Store.
type Store struct {
	coll *mongo.Collection
	opts StoreOptions
}

// Compile-time check.
var _ history.Store = &Store{}

// StoreOptions configures the MongoDB history store.
type StoreOptions struct {
	// CollectionNameFunc derives the collection name from the record type.
	// If set, a different collection is used per record type.
	// If nil, the single provided collection is used for all record types.
	CollectionNameFunc func(recordType string) string
}

// StoreOption is a functional option for configuring the Store.
type StoreOption func(*StoreOptions)

// WithCollectionNameFunc sets a function that derives collection names from record types.
func WithCollectionNameFunc(fn func(string) string) StoreOption {
	return func(o *StoreOptions) { o.CollectionNameFunc = fn }
}

// NewStore creates a new MongoDB-backed history store.
// The provided collection is the default; use WithCollectionNameFunc to route
// different record types to different collections.
func NewStore(coll *mongo.Collection, opts ...StoreOption) *Store {
	var o StoreOptions
	for _, fn := range opts {
		fn(&o)
	}
	return &Store{coll: coll, opts: o}
}

// collection returns the mongo.Collection for a given record type.
func (s *Store) collection(recordType string) *mongo.Collection {
	if s.opts.CollectionNameFunc != nil {
		name := s.opts.CollectionNameFunc(recordType)
		return s.coll.Database().Collection(name)
	}
	return s.coll
}

// EnsureIndexes creates recommended indexes for the change records collection.
// Call this once during application startup.
func (s *Store) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "record_type", Value: 1},
				{Key: "record_id", Value: 1},
				{Key: "version", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "parent_type", Value: 1},
				{Key: "parent_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "record_type", Value: 1},
				{Key: "created_at", Value: -1},
			},
		},
	}

	_, err := s.coll.Indexes().CreateMany(ctx, indexes)
	return err
}

// Record persists a change record.
func (s *Store) Record(ctx context.Context, record history.ChangeRecord) error {
	changesJSON, err := json.Marshal(record.Changes)
	if err != nil {
		return fmt.Errorf("r3hmongo: marshal changes: %w", err)
	}

	metaJSON, err := json.Marshal(record.Metadata)
	if err != nil {
		return fmt.Errorf("r3hmongo: marshal metadata: %w", err)
	}

	if record.ID == "" {
		record.ID = generateID()
	}

	doc := changeRecordDoc{
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

	coll := s.collection(record.RecordType)
	_, err = coll.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("r3hmongo: insert: %w", err)
	}

	return nil
}

// ForRecord returns history for a specific entity, ordered by version ascending.
func (s *Store) ForRecord(
	ctx context.Context,
	recordType string,
	recordID string,
	qarg ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	coll := s.collection(recordType)

	filter := bson.D{
		{Key: "record_type", Value: recordType},
		{Key: "record_id", Value: recordID},
	}

	sort := bson.D{{Key: "version", Value: 1}}

	return s.queryRecords(ctx, coll, filter, sort, qarg...)
}

// ForType returns history for all entities of a given type, ordered by created_at descending.
func (s *Store) ForType(
	ctx context.Context,
	recordType string,
	qarg ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	coll := s.collection(recordType)

	filter := bson.D{
		{Key: "record_type", Value: recordType},
	}

	sort := bson.D{{Key: "created_at", Value: -1}}

	return s.queryRecords(ctx, coll, filter, sort, qarg...)
}

// ForTree returns history across a parent-child hierarchy.
func (s *Store) ForTree(
	ctx context.Context,
	scopes []history.TreeScope,
	qarg ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	if len(scopes) == 0 {
		return nil, 0, nil
	}

	coll := s.collection(scopes[0].RecordType)

	var orConditions bson.A
	for _, scope := range scopes {
		cond := bson.D{{Key: "record_type", Value: scope.RecordType}}

		if scope.RecordID != "" {
			cond = append(cond, bson.E{Key: "record_id", Value: scope.RecordID})
		}
		if scope.ParentType != "" {
			cond = append(cond, bson.E{Key: "parent_type", Value: scope.ParentType})
			if scope.ParentID != "" {
				cond = append(cond, bson.E{Key: "parent_id", Value: scope.ParentID})
			}
		}

		orConditions = append(orConditions, cond)
	}

	filter := bson.D{{Key: "$or", Value: orConditions}}
	sort := bson.D{{Key: "created_at", Value: -1}}

	return s.queryRecords(ctx, coll, filter, sort, qarg...)
}

// NextVersion atomically returns the next version number.
func (s *Store) NextVersion(ctx context.Context, recordType string, recordID string) (int64, error) {
	coll := s.collection(recordType)

	filter := bson.D{
		{Key: "record_type", Value: recordType},
		{Key: "record_id", Value: recordID},
	}

	opts := options.FindOne().
		SetSort(bson.D{{Key: "version", Value: -1}}).
		SetProjection(bson.D{{Key: "version", Value: 1}})

	var result struct {
		Version int64 `bson:"version"`
	}

	err := coll.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 1, nil
		}
		return 0, fmt.Errorf("r3hmongo: next version: %w", err)
	}

	return result.Version + 1, nil
}

// GetVersion returns the change record for a specific version.
func (s *Store) GetVersion(
	ctx context.Context,
	recordType string,
	recordID string,
	version int64,
) (history.ChangeRecord, error) {
	coll := s.collection(recordType)

	filter := bson.D{
		{Key: "record_type", Value: recordType},
		{Key: "record_id", Value: recordID},
		{Key: "version", Value: version},
	}

	var doc changeRecordDoc
	err := coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return history.ChangeRecord{}, history.ErrVersionNotFound
		}
		return history.ChangeRecord{}, fmt.Errorf("r3hmongo: get version: %w", err)
	}

	return docToRecord(doc)
}

// LatestVersion returns the most recent change record.
func (s *Store) LatestVersion(ctx context.Context, recordType string, recordID string) (history.ChangeRecord, error) {
	coll := s.collection(recordType)

	filter := bson.D{
		{Key: "record_type", Value: recordType},
		{Key: "record_id", Value: recordID},
	}

	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}})

	var doc changeRecordDoc
	err := coll.FindOne(ctx, filter, opts).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return history.ChangeRecord{}, history.ErrNoHistory
		}
		return history.ChangeRecord{}, fmt.Errorf("r3hmongo: latest version: %w", err)
	}

	return docToRecord(doc)
}

// queryRecords is a shared helper for executing paginated queries.
func (s *Store) queryRecords(
	ctx context.Context,
	coll *mongo.Collection,
	filter bson.D,
	defaultSort bson.D,
	qarg ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	q := mergeQuery(qarg...)

	isPaginated := q.Pagination != nil && q.Pagination.IsPaginated()

	var totalCount int64
	if isPaginated {
		count, err := coll.CountDocuments(ctx, filter)
		if err != nil {
			return nil, 0, fmt.Errorf("r3hmongo: count: %w", err)
		}
		totalCount = count
		if totalCount == 0 {
			return nil, 0, nil
		}
	}

	findOpts := options.Find().SetSort(defaultSort)

	if isPaginated {
		limit, offset := q.Pagination.ToLimitOffset()
		findOpts.SetLimit(int64(limit))
		findOpts.SetSkip(int64(offset))
	}

	cursor, err := coll.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("r3hmongo: find: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []changeRecordDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, 0, fmt.Errorf("r3hmongo: decode: %w", err)
	}

	records := make([]history.ChangeRecord, 0, len(docs))
	for _, doc := range docs {
		rec, err := docToRecord(doc)
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

// docToRecord converts a BSON document to an history.ChangeRecord.
func docToRecord(doc changeRecordDoc) (history.ChangeRecord, error) {
	var record history.ChangeRecord

	record.ID = doc.ID
	record.RecordType = doc.RecordType
	record.RecordID = doc.RecordID
	record.Action = history.Action(doc.Action)
	record.Version = doc.Version
	record.ParentType = doc.ParentType
	record.ParentID = doc.ParentID
	record.CreatedAt = doc.CreatedAt

	if doc.Changes != "" {
		if err := json.Unmarshal([]byte(doc.Changes), &record.Changes); err != nil {
			return record, fmt.Errorf("r3hmongo: unmarshal changes: %w", err)
		}
	}

	if doc.Metadata != "" {
		if err := json.Unmarshal([]byte(doc.Metadata), &record.Metadata); err != nil {
			return record, fmt.Errorf("r3hmongo: unmarshal metadata: %w", err)
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
