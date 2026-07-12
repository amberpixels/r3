package validation

import (
	"context"
	"errors"
)

// NoValidation returns a validator that allows everything - for tests or
// prototyping when the decorator is in the chain but rules aren't ready.
func NoValidation[T any, ID comparable]() Validator[T, ID] {
	return ValidatorFunc[T, ID](func(_ context.Context, _ Request[T, ID]) error {
		return nil
	})
}

// Compose runs all validators, merging their *Error field errors into one
// [Error]. A non-*Error short-circuits and is returned immediately. An empty list
// allows everything.
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
