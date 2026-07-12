// Package validation validates entities before mutations (Create, Update, Patch,
// Upsert, PatchWhere) on any r3.CRUD[T, ID]. The decorator wraps the repo,
// transparently satisfies it, and drops in anywhere a CRUD is expected. Reads
// (Get, List) and Delete pass through unvalidated.
//
// R3 ships no validation rules. You implement the [Validator] interface with any
// library - [go-playground/validator], [go-ozzo/ozzo-validation], or plain Go;
// the decorator calls it and short-circuits with a structured [Error] on failure.
// Composable helpers: [NoValidation], [Compose], [OperationValidators].
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
// # State-Transition Validation
//
// With [WithIDFunc], the decorator fetches the existing entity before Update and
// Patch, so the validator receives req.Existing (current DB state) and can enforce
// rules like "status can only move from draft to published":
//
//	repo := validation.WithValidation[Post, int64](
//	    innerRepo, myValidator,
//	    validation.WithIDFunc[Post, int64](func(p Post) int64 { return p.ID }),
//	)
//
// # Patch-Aware Validation
//
// For Patch, [Request.Fields] lists the fields being changed, so validators can
// skip rules for unchanged ones:
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
// validate req.Entity directly - on a Patch it carries only the patched fields,
// the rest zeroed, so the rule would wrongly fire. When WithIDFunc is set, the
// request carries [Request.Merged] (the patch overlaid on current state) - validate
// that instead:
//
//	if req.Operation == validation.OpPatch && req.Merged != nil {
//	    return v.validate.Struct(*req.Merged)
//	}
//
// # Atomic State-Transition Validation
//
// The fetch-validate-write sequence is only atomic when the decorator runs inside
// a transaction: wrap the repo with the transactor feature and run Update/Patch in
// InTx so the read and write share one transaction. Otherwise a concurrent writer
// can change the row between fetch and write (a TOCTOU window), and two concurrent
// updates may both pass a state-transition rule.
//
// [go-playground/validator]: https://github.com/go-playground/validator
// [go-ozzo/ozzo-validation]: https://github.com/go-ozzo/ozzo-validation
package validation
