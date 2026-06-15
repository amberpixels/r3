// Package validation provides a decorator that validates entities before mutation
// operations (Create, Update, Patch) on any r3.CRUD[T, ID] repository.
//
// R3 does not include built-in validation rules. Instead, it provides a [Validator]
// interface that users implement using any validation library of their choice --
// [go-playground/validator], [go-ozzo/ozzo-validation], or plain Go code.
// The decorator intercepts mutations, calls the user's Validator, and short-circuits
// with a structured [Error] if validation fails.
//
// Read operations (Get, List) and Delete pass through without validation.
//
// Key features:
//   - Library-agnostic: bring your own validation (go-playground, ozzo, custom)
//   - Patch-aware: the validator receives which fields are being patched
//   - State-transition aware: optionally fetches the existing entity before Update/Patch
//   - Structured errors: [Error] carries per-field error details
//   - Composable helpers: [NoValidation], [Compose], [OperationValidators]
//
// # Basic Usage
//
//	repo := validation.WithValidation[Pet, int64](
//	    innerRepo,
//	    validation.ValidatorFunc[Pet, int64](func(ctx context.Context, req validation.Request[Pet, int64]) error {
//	        if req.Entity.Name == "" {
//	            return validation.NewError(req.Operation,
//	                validation.NewFieldError("name", "is required", "required"),
//	            )
//	        }
//	        return nil
//	    }),
//	)
//
// # With go-playground/validator
//
//	type petValidator struct {
//	    validate *validator.Validate
//	}
//
//	func (v *petValidator) Validate(_ context.Context, req validation.Request[Pet, int64]) error {
//	    if err := v.validate.Struct(req.Entity); err != nil {
//	        // Convert go-playground errors to r3 validation.Error
//	        var fieldErrors []validation.FieldError
//	        for _, fe := range err.(validator.ValidationErrors) {
//	            fieldErrors = append(fieldErrors, validation.NewFieldError(
//	                fe.Field(), fe.Tag()+" validation failed", fe.Tag(),
//	            ))
//	        }
//	        return validation.NewError(req.Operation, fieldErrors...)
//	    }
//	    return nil
//	}
//
// # With ozzo-validation
//
//	type petValidator struct{}
//
//	func (v petValidator) Validate(_ context.Context, req validation.Request[Pet, int64]) error {
//	    pet := req.Entity
//	    if err := ozzovalidation.ValidateStruct(&pet,
//	        ozzovalidation.Field(&pet.Name, ozzovalidation.Required, ozzovalidation.Length(1, 100)),
//	        ozzovalidation.Field(&pet.Price, ozzovalidation.Min(0.0)),
//	    ); err != nil {
//	        // Convert ozzo errors to r3 validation.Error
//	        var fieldErrors []validation.FieldError
//	        if ozzoErrs, ok := err.(ozzovalidation.Errors); ok {
//	            for field, fieldErr := range ozzoErrs {
//	                fieldErrors = append(fieldErrors, validation.NewFieldError(field, fieldErr.Error(), "invalid"))
//	            }
//	        }
//	        return validation.NewError(req.Operation, fieldErrors...)
//	    }
//	    return nil
//	}
//
// # State-Transition Validation
//
// When [WithIDFunc] is configured, the decorator fetches the existing entity before
// Update and Patch operations. This enables state-transition rules like "status can
// only move from draft to published":
//
//	repo := validation.WithValidation[Post, int64](
//	    innerRepo, myValidator,
//	    validation.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
//	)
//
// The validator then receives req.Existing with the current DB state.
//
// # Patch-Aware Validation
//
// For Patch operations, [Request.Fields] contains the list of fields being
// updated. Validators can use this to skip rules for unchanged fields:
//
//	func (v *myValidator) Validate(ctx context.Context, req validation.Request[Post, int64]) error {
//	    if req.Operation == validation.OpPatch {
//	        // Only validate fields that are being patched
//	        for _, f := range req.Fields {
//	            switch f.Name {
//	            case "name":
//	                if req.Entity.Name == "" {
//	                    return validation.NewError(req.Operation,
//	                        validation.NewFieldError("name", "is required", "required"),
//	                    )
//	                }
//	            }
//	        }
//	        return nil
//	    }
//	    // Full validation for Create/Update ...
//	}
//
// For whole-entity rules on Patch (e.g. "name must always be non-empty"), do NOT
// validate req.Entity directly — on a Patch it carries only the patched fields,
// with the rest zeroed, so the rule would wrongly fire. When WithIDFunc is set the
// request carries [Request.Merged]: the patch overlaid on the current DB state.
// Validate that instead:
//
//	if req.Operation == validation.OpPatch && req.Merged != nil {
//	    return v.validate.Struct(*req.Merged)
//	}
//
// # Atomic State-Transition Validation
//
// The decorator fetches the existing entity (or merged state) and then writes;
// that fetch-validate-write sequence is only atomic when the decorator runs inside
// a transaction. Wrap the repository with the transactor feature and perform the
// Update/Patch within InTx so the read and the write share one transaction.
// Outside a transaction a concurrent writer can change the row between the fetch
// and the write (a TOCTOU window), so two concurrent updates may both pass a
// state-transition rule.
//
// [go-playground/validator]: https://github.com/go-playground/validator
// [go-ozzo/ozzo-validation]: https://github.com/go-ozzo/ozzo-validation
package validation
