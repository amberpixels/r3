package r3

// Unwrapper is implemented by decorators that wrap an inner [CRUD]. Exposing the
// wrapped CRUD lets capability detection ([As]) and transaction propagation
// ([InTx], [BeginTxChain]) walk the decorator chain down to the backend.
//
// Every r3 feature decorator implements Unwrapper.
type Unwrapper[T any, ID comparable] interface {
	Unwrap() CRUD[T, ID]
}

// Rewrapper is implemented by decorators that can rebuild themselves around a
// different inner [CRUD]. It lets [InTx]/[BeginTxChain] re-apply the decorator
// stack on top of a transaction-bound CRUD, so decorated behaviour (validation,
// history, permissions, ...) still runs inside the transaction instead of being
// bypassed.
//
// Rewrap must return a decorator equivalent to the receiver but delegating to
// inner. Stateful decorators should share their backing state (stores, locks)
// with the rebuilt instance.
type Rewrapper[T any, ID comparable] interface {
	Rewrap(inner CRUD[T, ID]) CRUD[T, ID]
}

// As finds the first layer in the decorator chain (starting at repo and
// following [Unwrapper.Unwrap]) that implements C, enabling capability detection
// through decorators. For example, to reach a backend's soft-delete support
// regardless of how many decorators wrap it:
//
//	sd, ok := r3.As[SoftDeleter[ID]](repo)
//
// It returns the zero value of C and false if no layer implements C.
func As[C any, T any, ID comparable](repo CRUD[T, ID]) (C, bool) {
	cur := repo
	for {
		if c, ok := any(cur).(C); ok {
			return c, true
		}
		u, ok := any(cur).(Unwrapper[T, ID])
		if !ok {
			var zero C
			return zero, false
		}
		cur = u.Unwrap()
	}
}
