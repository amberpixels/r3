package r3

import (
	"fmt"

	"github.com/amberpixels/r3/internal/notimplemented"
)

// Preload defines a single preload rule.
type Preload interface {
	// Stringer is needed for debugging purposes, so each d can be printed.
	fmt.Stringer

	// Cloner is needed so the preloads are safe and immutable.
	Cloner[Preload]

	GetName() string             // The name of the related entity, e.g., "Author".
	GetNestedPreloads() Preloads // Nested preloads, e.g., "Author.Books".
}

// Preloads is a slice of Preload-s.
type Preloads []Preload

// MergeWith merges (combines) preloads with other preloads.
func (preloads Preloads) MergeWith(other Preloads) Preloads { return mergeWith(preloads, other) }

// Clone returns a cloned list of given preloads.
func (preloads Preloads) Clone() Preloads {
	clone := make(Preloads, len(preloads))
	for i, p := range preloads {
		clone[i] = p.Clone()
	}

	return clone
}

// Dedupe removes duplicates from the preloads list.
// Note: it's not super-performant because of types and go-generics. Refactor if needed.
func (preloads *Preloads) Dedupe() {
	v := []Preload(*preloads)
	dedupe(&v)
	*preloads = v
}

// PreloadSpec means a simple possible preload (name of a table/collection).
type PreloadSpec struct {
	Name string
}

// GetName returns the name of the current preload.
func (t *PreloadSpec) GetName() string { return t.Name }

// GetNestedPreloads returns list of nested prelaods from this preload.
func (t *PreloadSpec) GetNestedPreloads() Preloads {
	notimplemented.Panic("GetNestedPreloads will be implemented in future versions")
	return nil
}

// Clone returns a newly created struct of PreloadSpec.
func (t *PreloadSpec) Clone() Preload {
	return &PreloadSpec{Name: t.Name}
}

// String simply returns name of the preload.
func (t *PreloadSpec) String() string { return t.Name }

// NewPreloadSpec creates a PreloadSpec.
func NewPreloadSpec(name string) *PreloadSpec { return &PreloadSpec{Name: name} }
