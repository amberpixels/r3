package option

import (
	"encoding"
	"encoding/json"
	"fmt"
	"strings"
)

// Option is a safe alternative to a pointer to a value of type T.
// It represents a value that can either be present (Some) or absent (None).
// For example, for T = bool, it models an optional boolean value that can be true, false, or not specified.
type Option[T comparable] struct {
	value T
	ok    bool
}

// Bool is just a shortcut for Option[bool].
type Bool = Option[bool]

type Int = Option[int]

// None returns true if the Option does not contain a valid value.
func (o *Option[T]) None() bool {
	return !o.ok
}

// Some works in two ways:
//   - When called without arguments, it returns true if the Option contains a valid value.
//   - When called with one argument, it returns true if the Option contains a valid value
//     and that value is equal to the provided argument.
//
// Panics if more than one argument is provided.
func (o *Option[T]) Some(args ...T) bool {
	if len(args) == 0 {
		return o.ok
	} else if len(args) == 1 {
		return o.ok && o.value == args[0]
	}

	panic("Some accepts at most one argument")
}

// Unwrap returns the contained value if present; otherwise, it panics.
// This mirrors Rust's unwrap, providing a quick way to extract the value
// when you are certain that it is present.
func (o *Option[T]) Unwrap() T {
	if !o.ok {
		panic("called Unwrap on a None Option")
	}
	return o.value
}

// Some constructs an Option that contains a valid value.
func Some[T comparable](v T) Option[T] {
	return Option[T]{value: v, ok: true}
}

// None constructs an Option that does not contain a valid value.
func None[T comparable]() Option[T] {
	return Option[T]{ok: false}
}

func True() Option[bool]     { return Some(true) }
func False() Option[bool]    { return Some(false) }
func NoneBool() Option[bool] { return None[bool]() }

// MarshalJSON implements the json.Marshaler interface.
// If the Option is None, it marshals to JSON null; otherwise, it marshals to the contained value.
func (o Option[T]) MarshalJSON() ([]byte, error) {
	if !o.ok {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// If the JSON value is null, the Option is set to None; otherwise, it unmarshals into the contained value.
func (o *Option[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*o = None[T]()
		return nil
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.value = v
	o.ok = true
	return nil
}

// MarshalTOML returns the underlying value if it exists, or nil otherwise.
func (o Option[T]) MarshalTOML() ([]byte, error) {
	if o.ok {
		return json.Marshal(o.value)
	}

	return json.Marshal(TomlNone)
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// It interprets empty strings or "null" (case-insensitive) as a None value.
// Otherwise, it attempts to convert the text into type T.
func (o *Option[T]) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	if s == "" || strings.EqualFold(s, "null") || strings.EqualFold(s, TomlNone) {
		*o = None[T]()
		return nil
	}

	var v T
	// If T implements encoding.TextUnmarshaler, use it
	if tm, ok := any(&v).(encoding.TextUnmarshaler); ok {
		if err := tm.UnmarshalText(text); err != nil {
			return err
		}
		*o = Some(v)
		return nil
	}

	var isScalar bool
	switch any(v).(type) {
	case float64:
		isScalar = true
	case string:
		isScalar = true
	case bool:
		isScalar = true
	}

	// for scalar we can re-use json.Unmarshal handler
	if isScalar {
		var scalar T
		if err := json.Unmarshal(text, &scalar); err != nil {
			return err
		}
		*o = Some(scalar)
		return nil
	}

	return fmt.Errorf("type %T does not implement encoding.TextUnmarshaler and no fallback conversion is defined", v)
}

// IsZero returns true if Option is none (for `omitzero` interface)
func (o *Option[T]) IsZero() bool {
	if o.None() {
		return true
	}
	var zero T
	return o.Unwrap() == zero
}
