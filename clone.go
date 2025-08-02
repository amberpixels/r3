package r3

// Cloner is a wrapper for Clone() function that clones anything (identified via its interface T).
type Cloner[T any] interface {
	Clone() T
}
