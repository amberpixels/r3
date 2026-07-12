package metrics

import (
	"maps"
	"time"

	"github.com/amberpixels/r3"
)

// Labels attaches arbitrary key-value dimensions to a metric record for slicing
// and filtering (e.g. {"order_type": "manual", "actor_id": "42"}).
type Labels map[string]string

// Merge combines two label sets; other wins on key conflicts.
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

// MetricRecord is a single recorded metric data point, itself a first-class r3
// entity storable via any r3.CRUD[MetricRecord, string] backend.
type MetricRecord struct {
	// ID is the unique identifier for this record.
	ID string `json:"id" db:"id,pk" bson:"_id"`

	// RecordType is the entity type name (e.g. "users"), derived from the struct
	// type or set via options.
	RecordType string `json:"record_type" db:"record_type" bson:"record_type"`

	// RecordID is the stringified entity key; empty for type-level metrics.
	RecordID string `json:"record_id" db:"record_id" bson:"record_id"`

	// MetricName identifies the metric (e.g. "crud.action").
	MetricName string `json:"metric_name" db:"metric_name" bson:"metric_name"`

	// Value is the numeric measurement (1 for counters, ms for latency, etc.).
	Value float64 `json:"value" db:"value" bson:"value"`

	// Labels carries arbitrary dimensions, stored as a JSON blob in SQL.
	Labels r3.JSONColumn[Labels] `json:"labels" db:"labels" bson:"labels"`

	// Bucket is the pre-computed time bucket for efficient aggregation: RFC3339
	// truncated to the boundary (e.g. "2026-02-16T00:00:00Z" for daily).
	Bucket string `json:"bucket" db:"bucket" bson:"bucket"`

	// CreatedAt is when this metric was recorded.
	CreatedAt time.Time `json:"created_at" db:"created_at" bson:"created_at"`
}

// MetricEntry is what a Collector emits; the decorator enriches it with ID,
// bucket, timestamps, and merged labels before storing as a [MetricRecord].
type MetricEntry struct {
	// MetricName identifies the metric (e.g. "crud.action").
	MetricName string

	// Value is the numeric measurement.
	Value float64

	// Labels are collector-specific, merged with global/context/core labels by
	// the decorator.
	Labels Labels

	// RecordID is the stringified entity key; empty for type-level metrics.
	RecordID string
}
