package r3

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Sentinel errors for cursor pagination.
var (
	// ErrInvalidCursor is returned when a cursor token cannot be decoded.
	ErrInvalidCursor = errors.New("invalid cursor token")

	// ErrCursorRequiresSort is returned for cursor pagination without any sort
	// columns: keyset paging needs a stable order to page against.
	ErrCursorRequiresSort = errors.New("cursor pagination requires at least one sort column")
)

// CursorDirection indicates whether cursor pagination goes forward or backward.
type CursorDirection int8

const (
	// CursorForward pages forward from the "after" position.
	CursorForward CursorDirection = iota
	// CursorBackward pages backward from the "before" position.
	CursorBackward
)

// String returns "forward" or "backward".
func (d CursorDirection) String() string {
	if d == CursorBackward {
		return "backward"
	}
	return "forward"
}

// CursorValues holds the sort-column values (column name -> value) that define a
// cursor position.
type CursorValues map[string]any

// EncodeCursor serializes cursor values into an opaque base64 token.
func EncodeCursor(cv CursorValues) (string, error) {
	if len(cv) == 0 {
		return "", nil
	}
	data, err := json.Marshal(cv)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeCursor deserializes an opaque base64 token back into CursorValues.
// Returns empty CursorValues (not nil) for an empty token.
func DecodeCursor(token string) (CursorValues, error) {
	if token == "" {
		return CursorValues{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode: %w", ErrInvalidCursor, err)
	}
	var cv CursorValues
	if err := json.Unmarshal(data, &cv); err != nil {
		return nil, fmt.Errorf("%w: json unmarshal: %w", ErrInvalidCursor, err)
	}
	return cv, nil
}

// CursorSpec specifies cursor/keyset pagination. Set at most one of After or
// Before; if both are set, After wins.
type CursorSpec struct {
	// After is the opaque cursor token for forward pagination.
	After string
	// Before is the opaque cursor token for backward pagination.
	Before string
	// Limit is the max results to return; 0 means the default.
	Limit int
}

// NewCursorAfter creates a CursorSpec for forward pagination after the token.
func NewCursorAfter(after string, limit int) *CursorSpec {
	return &CursorSpec{After: after, Limit: limit}
}

// NewCursorBefore creates a CursorSpec for backward pagination before the token.
func NewCursorBefore(before string, limit int) *CursorSpec {
	return &CursorSpec{Before: before, Limit: limit}
}

// NewCursorFirst creates a CursorSpec for the first page (limit only, no cursor).
func NewCursorFirst(limit int) *CursorSpec {
	return &CursorSpec{Limit: limit}
}

// Direction returns CursorForward or CursorBackward based on which token is set.
func (c *CursorSpec) Direction() CursorDirection {
	if c.Before != "" && c.After == "" {
		return CursorBackward
	}
	return CursorForward
}

// Token returns the active cursor token (After takes precedence over Before).
func (c *CursorSpec) Token() string {
	if c.After != "" {
		return c.After
	}
	return c.Before
}

// GetLimit returns the limit, defaulting to [PageSizeDefault] if unset.
func (c *CursorSpec) GetLimit() int {
	if c.Limit > 0 {
		return c.Limit
	}
	return PageSizeDefault
}

// String returns a debug-friendly representation.
func (c *CursorSpec) String() string {
	if c == nil {
		return "no_cursor"
	}
	if c.After != "" {
		return fmt.Sprintf("after=%s,limit=%d", c.After, c.GetLimit())
	}
	if c.Before != "" {
		return fmt.Sprintf("before=%s,limit=%d", c.Before, c.GetLimit())
	}
	return fmt.Sprintf("first,limit=%d", c.GetLimit())
}

// Clone returns a deep copy of the CursorSpec.
func (c *CursorSpec) Clone() *CursorSpec {
	if c == nil {
		return nil
	}
	clone := *c
	return &clone
}

// MergeWith merges this cursor spec with another, with other taking precedence.
func (c *CursorSpec) MergeWith(other *CursorSpec) *CursorSpec {
	if other == nil {
		return c.Clone()
	}
	if c == nil {
		return other.Clone()
	}
	result := c.Clone()
	if other.After != "" {
		result.After = other.After
	}
	if other.Before != "" {
		result.Before = other.Before
	}
	if other.Limit > 0 {
		result.Limit = other.Limit
	}
	return result
}

// FinalizeCountCursor returns (entities, -1): keyset pagination has no total count.
func FinalizeCountCursor[T any](entities []T) ([]T, int64) {
	return entities, -1
}
