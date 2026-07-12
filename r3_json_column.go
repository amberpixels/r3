package r3

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONColumn wraps a value of type T so it persists as a JSON string in a single
// SQL column while staying transparent to JSON APIs: it implements sql.Scanner,
// driver.Valuer, json.Marshaler, and json.Unmarshaler. Use it for fields stored as
// a JSON blob (e.g. []FieldChange, metadata maps, nested config).
type JSONColumn[T any] struct {
	Val T
}

// NewJSONColumn creates a JSONColumn wrapping the given value.
func NewJSONColumn[T any](v T) JSONColumn[T] {
	return JSONColumn[T]{Val: v}
}

// Value implements driver.Valuer — marshals T to a JSON string for SQL storage.
func (j JSONColumn[T]) Value() (driver.Value, error) {
	data, err := json.Marshal(j.Val)
	if err != nil {
		return nil, fmt.Errorf("r3: JSONColumn marshal: %w", err)
	}
	return string(data), nil
}

// Scan implements sql.Scanner — reads a JSON string (or []byte) from SQL and unmarshals into T.
func (j *JSONColumn[T]) Scan(src any) error {
	if src == nil {
		var zero T
		j.Val = zero
		return nil
	}

	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("r3: JSONColumn scan: unsupported type %T", src)
	}

	return json.Unmarshal(data, &j.Val)
}

// MarshalJSON implements json.Marshaler — transparent JSON serialization of the wrapped value.
func (j JSONColumn[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Val)
}

// UnmarshalJSON implements json.Unmarshaler — transparent JSON deserialization into the wrapped value.
func (j *JSONColumn[T]) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.Val)
}
