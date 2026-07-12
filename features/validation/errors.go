package validation

import (
	"errors"
	"fmt"
	"strings"
)

// ErrValidation is the sentinel every validation failure matches via errors.Is.
var ErrValidation = errors.New("r3/validation: validation failed")

// FieldError describes a validation failure for a single field.
type FieldError struct {
	// Field that failed (e.g. "name", "email").
	Field string

	// Message is the human-readable failure (e.g. "is required").
	Message string

	// Code is a machine-readable code (e.g. "required", "min") for i18n or
	// client-side error mapping.
	Code string
}

// Error returns a human-readable representation of the field error.
func (e FieldError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// NewFieldError creates a new FieldError.
func NewFieldError(field, message, code string) FieldError {
	return FieldError{
		Field:   field,
		Message: message,
		Code:    code,
	}
}

// Error carries structured per-field validation failures and matches
// errors.Is(err, ErrValidation) via its Is method.
type Error struct {
	// Operation is the mutation that was attempted.
	Operation Operation

	// Errors is the list of per-field failures.
	Errors []FieldError
}

// Error returns a human-readable summary of all validation failures.
func (e *Error) Error() string {
	if len(e.Errors) == 0 {
		return fmt.Sprintf("r3/validation: validation failed on %s", e.Operation)
	}

	msgs := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		msgs[i] = fe.Error()
	}

	return fmt.Sprintf("r3/validation: validation failed on %s: %s",
		e.Operation, strings.Join(msgs, "; "))
}

// Is supports errors.Is(err, ErrValidation).
func (e *Error) Is(target error) bool {
	return target == ErrValidation
}

// HasField reports whether the validation error contains a failure for the named field.
func (e *Error) HasField(name string) bool {
	for _, fe := range e.Errors {
		if fe.Field == name {
			return true
		}
	}
	return false
}

// NewError creates a new Error with the given field errors.
func NewError(op Operation, errs ...FieldError) *Error {
	return &Error{
		Operation: op,
		Errors:    errs,
	}
}
