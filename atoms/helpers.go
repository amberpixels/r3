package r3atoms

import "github.com/amberpixels/r3/internal/gx"

// mergeWith merges (combines) things A with things B
func mergeWith[T comparable](a, b []T) []T {
	seen := gx.NewLookup(a...)

	// First step: let's count so we can pre-allocate
	var nAddendum int
	for _, e := range b {
		if !seen.Has(e) {
			nAddendum++
		}
	}

	// Second step: make the result from a + those from b that are not in a
	// As we calculate nAddendum, we won't use `append()` but will
	// instead use: make + copy and then filling via [i]
	n := len(a)
	result := make([]T, n+nAddendum)
	copy(result, a)

	j := n
	for _, e := range b {
		if !seen.Has(e) {
			result[j] = e
			j++
		}
	}

	return result
}

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
	return
}
