package gx

// Lookup is a lookup map by given key.
type Lookup[T comparable] map[T]struct{}

// Has returns true if lookup has given key.
func (l Lookup[T]) Has(k T) bool {
	_, ok := l[k]
	return ok
}

// Add adds an element into lookup.
func (l Lookup[T]) Add(k T) {
	l[k] = struct{}{}
}

// Delete deletes an element from the lookup.
func (l Lookup[T]) Delete(k T) {
	delete(l, k)
}

func (l Lookup[T]) Clear() {
	for k := range l {
		delete(l, k)
	}
}

// NewLookup returns new ready to use lookup map.
func NewLookup[T comparable](initialKeys ...T) Lookup[T] {
	l := make(Lookup[T], len(initialKeys))
	for _, key := range initialKeys {
		l.Add(key)
	}
	return l
}

// NewLookupCapped returns new ready to use lookup map with given map capacity
func NewLookupCapped[T comparable](n int, initialKeys ...T) Lookup[T] {
	l := make(Lookup[T], n)
	for _, key := range initialKeys {
		l.Add(key)
	}
	return l
}
