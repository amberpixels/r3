package r3

// Preloads is a slice of *PreloadSpec.
type Preloads []*PreloadSpec

// MergeWith merges (combines) preloads with other preloads.
func (preloads Preloads) MergeWith(other Preloads) Preloads { return mergeWith(preloads, other) }

// Dedupe removes duplicates from the preloads list.
func (preloads *Preloads) Dedupe() {
	v := []*PreloadSpec(*preloads)
	dedupe(&v)
	*preloads = v
}

// Clone returns a cloned list of given preloads.
func (preloads Preloads) Clone() Preloads {
	cloned := make(Preloads, len(preloads))
	for i, p := range preloads {
		cloned[i] = p.Clone()
	}
	return cloned
}

// PreloadSpec means a simple possible preload (name of a table/collection).
type PreloadSpec struct {
	Name string
}

// GetName returns the name of the current preload.
func (t *PreloadSpec) GetName() string { return t.Name }

// Clone returns a newly created struct of PreloadSpec.
func (t *PreloadSpec) Clone() *PreloadSpec {
	return &PreloadSpec{Name: t.Name}
}

// String simply returns name of the preload.
func (t *PreloadSpec) String() string { return t.Name }

// NewPreloadSpec creates a PreloadSpec.
func NewPreloadSpec(name string) *PreloadSpec { return &PreloadSpec{Name: name} }
