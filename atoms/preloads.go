package r3atoms

// Preload defines a single preload rule.
type Preload interface {
	GetName() string            // The name of the related entity, e.g., "Author".
	GetNestedPreloads() Preload // Nested preloads, e.g., "Author.Books".
}

// Preloads is a slice of Preload-s.
type Preloads []Preload

// EntityPreload means a simple Table/Collection preload
type EntityPreload struct {
	Name string
}

func (t *EntityPreload) GetName() string { return t.Name }

func (t *EntityPreload) GetNestedPreloads() Preload { return nil }

func NewEntityPreload(name string) *EntityPreload { return &EntityPreload{Name: name} }
