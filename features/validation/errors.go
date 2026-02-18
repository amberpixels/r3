package validation

import (
	"errors"
	"fmt"
	"strings"
)

// ErrValidation is the sentinel error for validation failures.
// Use errors.Is(err, validation.ErrValidation) to detect validation errors.
var ErrValidation = errors.New("r3/validation: validation failed")

// FieldError describes a validation failure for a single field.
type FieldError struct {
	// Field is the name of the field that failed validation (e.g. "name", "email").
	Field string

	// Message is a human-readable description of the failure (e.g. "is required").
	Message string

	// Code is a machine-readable error code (e.g. "required", "min", "email").
	// Useful for i18n or client-side error mapping.
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

// Error provides structured details about validation failures.
// It satisfies errors.Is(err, ErrValidation) via its Is method.
type Error struct {
	// Operation is the mutation that was attempted.
	Operation Operation

	// Errors is the list of per-field validation failures.
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
