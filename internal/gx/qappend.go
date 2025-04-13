package gx

// Qappend stands for QuickAppend (quick version of Append if not exists).
// It does work about 200% faster than native `if !slices.Contains() { append() }`
//
// Qappend DOESN'T make NEW duplicates. (But it respects the old duplicates)
func Qappend[T comparable](a []T, b ...T) []T {
	m := len(b)
	if m == 0 {
		return a
	}

	// N stands for the number of elements in A. We don't care if duplicates are there. We respect them.
	n := len(a)
	// seen stands for a unique set of elements in A (will be used for lookup when appending Bs)
	seen := NewLookup(a...)

	// Let's check if given b was "dirty" (with duplicates)
	// if Yes, then let's clean it up.
	// While doing so, we can count fresh elements
	if m0 := len(NewLookup(b...)); m0 != m {
		tmp := NewLookup[T]()
		// Fresh elements are elements from B that will be appended (not presented in A)
		nFreshElements, j := 0, 0
		newB := make([]T, m0)
		for _, e := range b {
			if !tmp.Has(e) {
				if !seen.Has(e) {
					nFreshElements++
				}

				newB[j] = e
				tmp.Add(e)
				j++
			}
		}
		b = newB
		m = nFreshElements
	} else {
		// let's count so we can pre-allocate
		tmp := NewLookup[T]()
		for _, e := range b {
			if !seen.Has(e) && !tmp.Has(e) {
				tmp.Add(e)
			}
		}
		m = len(tmp)
	}

	// Second step: make the result from a + those from b that are not in a
	// As we calculate nAddendum, we won't use `append()` but will
	// instead use: make + copy and then filling via [i]
	result := make([]T, n+m)
	copy(result, a)

	tmp, j := NewLookup[T](), 0
	for _, e := range b {
		if !seen.Has(e) && !tmp.Has(e) {
			result[n+j] = e
			tmp.Add(e)
			j++
		}
	}

	return result
}
