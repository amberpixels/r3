package r3

import "github.com/amberpixels/r3/internal/gx"

// mergeWith merges (combines) things A with things B.
func mergeWith[T comparable](a, b []T) []T {
	return gx.Qappend(a, b...)
}

// dedupe removes duplicates from the things list.
func dedupe[T comparable](things *[]T) {
	if things == nil {
		return
	}

	seen := gx.NewLookup[T]()
	idx := 0
	for _, thing := range *things {
		if !seen.Has(thing) {
			(*things)[idx] = thing
			idx++
			seen.Add(thing)
		}
	}

	// truncate the rest
	*things = (*things)[:idx]
}
