package r3

// Patchable defines a single patch (field + its value)
type Patchable interface {
	// GetField returns the field that should be patched
	GetField() Fieldable

	// GetValue returns the value of the patching field
	GetValue() any
}

// Patchables defines collection of patchables
type Patchables interface {
	GetPatches() []Patchable
}

type Patch struct {
	field Fieldable
	value any
}

func (p *Patch) GetField() Fieldable { return p.field }
func (p *Patch) GetValue() any       { return p.value }

func NewPatch(field Fieldable, value any) *Patch {
	return &Patch{field, value}
}
