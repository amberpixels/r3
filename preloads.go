package r3

import "github.com/amberpixels/r3/internal/notimplemented"

// Preload defines a single preload rule.
type Preload interface {
	GetName() string             // The name of the related entity, e.g., "Author".
	GetNestedPreloads() Preloads // Nested preloads, e.g., "Author.Books".
}

// Preloads is a slice of Preload-s.
type Preloads []Preload

// MergeWith merges (combines) preloads with other preloads.
func (preloads Preloads) MergeWith(other Preloads) Preloads { return mergeWith(preloads, other) }

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

func (t *PreloadSpec) GetName() string { return t.Name }

func (t *PreloadSpec) GetNestedPreloads() Preloads {
	notimplemented.Panic("GetNestedPreloads will be implemented in future versions")
	return nil
}

// NewPreloadSpec creates a PreloadSpec.
func NewPreloadSpec(name string) *PreloadSpec { return &PreloadSpec{Name: name} }
