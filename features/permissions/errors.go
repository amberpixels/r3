package permissions

import (
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
)

// ErrAccessDenied is the sentinel error for permission checks.
// Use errors.Is(err, permissions.ErrAccessDenied) to detect authorization failures.
var ErrAccessDenied = errors.New("r3/permissions: access denied")

// AccessDeniedError provides structured details about a denied operation.
// It satisfies errors.Is(err, ErrAccessDenied) via its Is method.
type AccessDeniedError struct {
	Operation  Operation
	Actor      r3.Actor
	RecordType string
	RecordID   string // empty if not applicable
	Reason     string // optional human-readable reason
}

// Error returns a human-readable description of the denied operation.
func (e *AccessDeniedError) Error() string {
	msg := fmt.Sprintf("r3/permissions: access denied: %s on %s", e.Operation, e.RecordType)
	if e.RecordID != "" {
		msg += fmt.Sprintf(" (id=%s)", e.RecordID)
	}
	msg += fmt.Sprintf(" by actor %s/%s", e.Actor.Type, e.Actor.ID)
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	return msg
}

// Is supports errors.Is(err, ErrAccessDenied).
func (e *AccessDeniedError) Is(target error) bool {
	return target == ErrAccessDenied
}
