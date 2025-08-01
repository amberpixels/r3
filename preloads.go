package r3

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
	dedupe[Preload](&v)
	*preloads = v
}

// EntityPreload means a simple Table/Collection preload.
type EntityPreload struct {
	Name string
}

func (t *EntityPreload) GetName() string { return t.Name }

func (t *EntityPreload) GetNestedPreloads() Preloads { return nil /* TODO(future) */ }

func NewEntityPreload(name string) *EntityPreload { return &EntityPreload{Name: name} }
