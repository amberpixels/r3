package r3atoms

// Preload defines a single preload rule.
type Preload interface {
	GetName() string            // The name of the related entity, e.g., "Author".
	GetNestedPreloads() Preload // Nested preloads, e.g., "Author.Books".
}

// Preloads is a slice of Preload-s.
type Preloads []Preload

// MergeWith merges (combines) preloads with other preloads
func (preloads Preloads) MergeWith(other Preloads) Preloads { return mergeWith(preloads, other) }

// Dedupe removes duplicates from the preloads list
// Not: it's not super-performant because of types and go-generics. Refactor if needed
func (fs *Preloads) Dedupe() {
	v := []Preload(*fs)
	dedupe[Preload](&v)
	*fs = Preloads(v)
}

// EntityPreload means a simple Table/Collection preload
type EntityPreload struct {
	Name string
}

func (t *EntityPreload) GetName() string { return t.Name }

func (t *EntityPreload) GetNestedPreloads() Preload { return nil }

func NewEntityPreload(name string) *EntityPreload { return &EntityPreload{Name: name} }
