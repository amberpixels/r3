package r3

// Preloadable defines a single preload rule.
type Preloadable interface {
	GetName() string                // The name of the related entity, e.g., "Author".
	GetNestedPreloads() Preloadable // Nested preloads, e.g., "Author.Books".
}

// Preloadables is a slice of Preloadable.
type Preloadables []Preloadable

type TablePreload struct {
	Name string
}

func (t *TablePreload) GetName() string { return t.Name }

func (t *TablePreload) GetNestedPreloads() Preloadable { return nil }

func NewTablePreload(name string) *TablePreload { return &TablePreload{Name: name} }
