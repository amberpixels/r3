package r3

// Preloads is a slice of *PreloadSpec.
//
//nolint:recvcheck // slice type: value receivers for non-mutating MergeWith/Clone, pointer only for in-place Dedupe; unifying would break non-addressable call sites
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

// PreloadSpec names a relation to preload (a table/collection name).
type PreloadSpec struct {
	Name string
}

// NewPreloadSpec creates a PreloadSpec.
func NewPreloadSpec(name string) *PreloadSpec { return &PreloadSpec{Name: name} }

// GetName returns the preload name.
func (t *PreloadSpec) GetName() string { return t.Name }

// Clone returns a copy of the preload.
func (t *PreloadSpec) Clone() *PreloadSpec {
	return &PreloadSpec{Name: t.Name}
}

// String returns the preload name.
func (t *PreloadSpec) String() string { return t.Name }
