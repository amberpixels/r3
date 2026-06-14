package enginefile

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/amberpixels/r3"
)

// IDGenerator generates new unique IDs for entities.
// The Generate method receives the list of existing IDs so that
// implementations can avoid collisions (e.g. auto-increment).
type IDGenerator[ID comparable] interface {
	Generate(existing []ID) (ID, error)
}

// IDGeneratorFunc is a function adapter for IDGenerator.
type IDGeneratorFunc[ID comparable] func(existing []ID) (ID, error)

// Generate implements IDGenerator by calling the function.
func (f IDGeneratorFunc[ID]) Generate(existing []ID) (ID, error) {
	return f(existing)
}

// IncrementIDGen returns an IDGenerator that picks max(existing) + 1.
// Works with integer types (int, int64, etc.).
type incrementIDGen[ID interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}] struct{}

func (g incrementIDGen[ID]) Generate(existing []ID) (ID, error) {
	var maxID ID
	for _, id := range existing {
		if id > maxID {
			maxID = id
		}
	}
	return maxID + 1, nil
}

// IncrementIDGen returns an IDGenerator that auto-increments from the highest existing ID.
func IncrementIDGen[ID interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}]() IDGenerator[ID] {
	return incrementIDGen[ID]{}
}

// UUIDStringIDGen returns an IDGenerator that generates UUID v4 strings.
// This uses crypto/rand and does not depend on any external UUID library.
func UUIDStringIDGen() IDGenerator[string] {
	return IDGeneratorFunc[string](func(_ []string) (string, error) {
		return generateUUIDv4()
	})
}

// UUID v4 bit masks.
const (
	uuidV4VersionMask = 0x0f // clears high nibble of byte 6
	uuidV4Version     = 0x40 // version 4
	uuidV4VariantMask = 0x3f // clears top 2 bits of byte 8
	uuidV4Variant     = 0x80 // variant 10
)

// generateUUIDv4 generates a UUID v4 string using crypto/rand.
func generateUUIDv4() (string, error) {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	uuid[6] = (uuid[6] & uuidV4VersionMask) | uuidV4Version
	uuid[8] = (uuid[8] & uuidV4VariantMask) | uuidV4Variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16]), nil
}

// extractIDs extracts primary key values from a slice of entities.
func extractIDs[T any, ID comparable](entities []T, meta *StructMeta) []ID {
	ids := make([]ID, 0, len(entities))
	for _, e := range entities {
		if v := meta.PKValue(e); v != nil {
			if id, ok := v.(ID); ok {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// errNotFound is returned when an entity is not found. It aliases r3.ErrNotFound
// so the file engine surfaces the same sentinel as every other backend.
var errNotFound = r3.ErrNotFound

// IsNotFound returns true if the error indicates that an entity was not found.
func IsNotFound(err error) bool {
	return errors.Is(err, errNotFound)
}
