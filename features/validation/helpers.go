package validation

import (
	"context"
	"errors"
)

// NoValidation returns a validator that allows everything.
// Useful for testing or prototyping when you want the decorator in the chain
// but don't have validation rules yet.
func NoValidation[T any, ID comparable]() Validator[T, ID] {
	return ValidatorFunc[T, ID](func(_ context.Context, _ Request[T, ID]) error {
		return nil
	})
}

// Compose chains multiple validators. All validators are called, and their
// errors are collected into a single [Error]. If any validator returns
// a *Error, the field errors are merged. If a validator returns a
// non-Error, it is returned immediately (short-circuit).
//
// An empty list of validators allows everything.
func Compose[T any, ID comparable](validators ...Validator[T, ID]) Validator[T, ID] {
	return ValidatorFunc[T, ID](func(ctx context.Context, req Request[T, ID]) error {
		var collected []FieldError

		for _, v := range validators {
			if err := v.Validate(ctx, req); err != nil {
				var ve *Error
				if isError(err, &ve) {
					collected = append(collected, ve.Errors...)
				} else {
					// Non-validation error: short-circuit immediately
					return err
				}
			}
		}

		if len(collected) > 0 {
			return NewError(req.Operation, collected...)
		}
		return nil
	})
}

// OperationValidators maps specific operations to specific validators.
// Unmapped operations pass through without validation.
//
// Example:
//
//	validation.OperationValidators[Pet, int64](map[validation.Operation]validation.Validator[Pet, int64]{
//	    validation.OpCreate: createValidator,
//	    validation.OpUpdate: updateValidator,
//	    // OpPatch not mapped -> passes through
//	})
func OperationValidators[T any, ID comparable](
	perOp map[Operation]Validator[T, ID],
) Validator[T, ID] {
	return ValidatorFunc[T, ID](func(ctx context.Context, req Request[T, ID]) error {
		if v, ok := perOp[req.Operation]; ok {
			return v.Validate(ctx, req)
		}
		return nil // unmapped operations pass through
	})
}

// isError checks if err is a *Error (using errors.As for wrapped errors)
// and assigns it to target.
func isError(err error, target **Error) bool {
	return errors.As(err, target)
}
