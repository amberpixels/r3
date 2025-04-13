package r3

// Patch defines a single patch (field + its value).
type Patch interface {
	// GetField returns the field that should be patched
	GetField() Field

	// GetValue returns the value of the patching field
	GetValue() any
}

type Patches []Patch

type ColumnPatch struct {
	field Field
	value any
}

func (p *ColumnPatch) GetField() Field { return p.field }
func (p *ColumnPatch) GetValue() any   { return p.value }

func NewPatch(field Field, value any) *ColumnPatch {
	return &ColumnPatch{field, value}
}
