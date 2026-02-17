package permissions

import "context"

// AllowAll returns a checker that allows everything.
func AllowAll[T any, ID comparable]() Checker[T, ID] {
	return CheckerFunc[T, ID](func(_ context.Context, _ AccessRequest[T, ID]) error {
		return nil
	})
}

// DenyAll returns a checker that denies everything.
func DenyAll[T any, ID comparable]() Checker[T, ID] {
	return CheckerFunc[T, ID](func(_ context.Context, req AccessRequest[T, ID]) error {
		return &AccessDeniedError{
			Operation: req.Operation,
			Actor:     req.Actor,
			Reason:    "all operations are denied",
		}
	})
}

// ReadOnly returns a checker that allows only reads (Get, List) and
// denies all mutations (Create, Update, Patch, Delete).
func ReadOnly[T any, ID comparable]() Checker[T, ID] {
	return CheckerFunc[T, ID](func(_ context.Context, req AccessRequest[T, ID]) error {
		if req.Operation == OpRead {
			return nil
		}
		return &AccessDeniedError{
			Operation: req.Operation,
			Actor:     req.Actor,
			Reason:    "read-only access",
		}
	})
}

// ByActorType returns a checker that delegates to different checkers
// based on the actor's Type field. If no checker is registered for the
// actor's type, the operation is denied.
func ByActorType[T any, ID comparable](rules map[string]Checker[T, ID]) Checker[T, ID] {
	return CheckerFunc[T, ID](func(ctx context.Context, req AccessRequest[T, ID]) error {
		checker, ok := rules[req.Actor.Type]
		if !ok {
			return &AccessDeniedError{
				Operation: req.Operation,
				Actor:     req.Actor,
				Reason:    "no rules for actor type: " + req.Actor.Type,
			}
		}
		return checker.Check(ctx, req)
	})
}

// Compose chains multiple checkers: all must allow for the operation to proceed.
// First denial wins. An empty list of checkers allows everything.
func Compose[T any, ID comparable](checkers ...Checker[T, ID]) Checker[T, ID] {
	return CheckerFunc[T, ID](func(ctx context.Context, req AccessRequest[T, ID]) error {
		for _, c := range checkers {
			if err := c.Check(ctx, req); err != nil {
				return err
			}
		}
		return nil
	})
}

// OperationCheckers maps specific operations to specific checkers.
// Unmapped operations are denied by default unless a fallback checker
// is provided.
//
// Example:
//
//	permissions.OperationCheckers[Post, int64](
//	    map[permissions.Operation]permissions.Checker[Post, int64]{
//	        permissions.OpRead:   permissions.AllowAll[Post, int64](),
//	        permissions.OpCreate: adminOnlyChecker,
//	    },
//	    permissions.DenyAll[Post, int64](), // fallback for unmapped ops
//	)
func OperationCheckers[T any, ID comparable](
	perOp map[Operation]Checker[T, ID],
	fallback ...Checker[T, ID],
) Checker[T, ID] {
	return CheckerFunc[T, ID](func(ctx context.Context, req AccessRequest[T, ID]) error {
		if checker, ok := perOp[req.Operation]; ok {
			return checker.Check(ctx, req)
		}
		if len(fallback) > 0 {
			return fallback[0].Check(ctx, req)
		}
		return &AccessDeniedError{
			Operation: req.Operation,
			Actor:     req.Actor,
			Reason:    "no checker configured for operation: " + string(req.Operation),
		}
	})
}
