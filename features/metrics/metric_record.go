package metrics

import (
	"maps"
	"time"

	"github.com/amberpixels/r3"
)

// Labels is a map of key-value string pairs attached to metric records.
// Labels enable slicing and filtering metrics by arbitrary dimensions
// (e.g. {"order_type": "manual", "actor_id": "42"}).
type Labels map[string]string

// Merge combines two label sets. Values from other take precedence on key conflicts.
func (l Labels) Merge(other Labels) Labels {
	if len(l) == 0 && len(other) == 0 {
		return nil
	}
	result := make(Labels, len(l)+len(other))
	maps.Copy(result, l)
	maps.Copy(result, other)
	return result
}

// Clone returns a shallow copy of the labels map.
func (l Labels) Clone() Labels {
	if l == nil {
		return nil
	}
	c := make(Labels, len(l))
	maps.Copy(c, l)
	return c
}

// MetricRecord represents a single recorded metric data point.
// It is a first-class r3 entity: it can be stored via any
// r3.CRUD[MetricRecord, string] implementation (SQL, GORM, MongoDB, etc.).
//
// Everything is a R3po.
type MetricRecord struct {
	// ID is the unique identifier for this metric record.
	ID string `json:"id" db:"id,pk" bson:"_id"`

	// RecordType is the entity type name (e.g. "users", "orders").
	// Derived automatically from the struct type or set explicitly via options.
	RecordType string `json:"record_type" db:"record_type" bson:"record_type"`

	// RecordID is the primary key of the entity, stringified.
	// Empty for type-level metrics (e.g. list counts).
	RecordID string `json:"record_id" db:"record_id" bson:"record_id"`

	// MetricName identifies the metric (e.g. "crud.action", "entity.popularity").
	MetricName string `json:"metric_name" db:"metric_name" bson:"metric_name"`

	// Value is the numeric measurement (1 for counters, ms for latency, etc.).
	Value float64 `json:"value" db:"value" bson:"value"`

	// Labels carries arbitrary key-value dimensions for slicing metrics.
	// Stored as a JSON blob in SQL databases via JSONColumn.
	Labels r3.JSONColumn[Labels] `json:"labels" db:"labels" bson:"labels"`

	// Bucket is a pre-computed time bucket string for efficient aggregation.
	// Format: ISO8601 truncated to bucket boundary (e.g. "2026-02-16T00:00:00Z" for daily).
	Bucket string `json:"bucket" db:"bucket" bson:"bucket"`

	// CreatedAt is the timestamp when this metric was recorded.
	CreatedAt time.Time `json:"created_at" db:"created_at" bson:"created_at"`
}

// MetricEntry is what a Collector emits before persistence.
// The decorator enriches it with ID, bucket, timestamps, and merged labels
// before storing as a MetricRecord.
type MetricEntry struct {
	// MetricName identifies the metric (e.g. "crud.action", "entity.popularity").
	MetricName string

	// Value is the numeric measurement.
	Value float64

	// Labels are collector-specific labels (merged with global/context/core labels by the decorator).
	Labels Labels

	// RecordID is the entity's primary key (stringified). Empty for type-level metrics.
	RecordID string
}
