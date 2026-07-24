package validation_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/expectto/be"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/validation"
)

// ── Test entity ──────────────────────────────────────────────────────────

type Pet struct {
	ID     int64
	Name   string
	Status string
	Price  float64
}

// ── In-memory CRUD mock ──────────────────────────────────────────────────

type memoryCRUD struct {
	mu     sync.Mutex
	data   map[int64]Pet
	nextID int64

	// Track calls for assertions.
	createCalls int
	getCalls    int
	listCalls   int
	updateCalls int
	patchCalls  int
	deleteCalls int
}

func newMemoryCRUD() *memoryCRUD {
	return &memoryCRUD{data: make(map[int64]Pet), nextID: 1}
}

func (m *memoryCRUD) Create(_ context.Context, entity Pet) (Pet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	entity.ID = m.nextID
	m.nextID++
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Get(_ context.Context, id int64, _ ...r3.Query) (Pet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	entity, ok := m.data[id]
	if !ok {
		return Pet{}, fmt.Errorf("not found: %d", id)
	}
	return entity, nil
}

func (m *memoryCRUD) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, qarg...)
	return n, err
}

func (m *memoryCRUD) List(_ context.Context, _ ...r3.Query) ([]Pet, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listCalls++
	var result []Pet
	for _, v := range m.data {
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *memoryCRUD) Update(_ context.Context, entity Pet) (Pet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls++
	if _, ok := m.data[entity.ID]; !ok {
		return Pet{}, fmt.Errorf("not found: %d", entity.ID)
	}
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Patch(_ context.Context, entity Pet, fields r3.Fields) (Pet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patchCalls++
	existing, ok := m.data[entity.ID]
	if !ok {
		return Pet{}, fmt.Errorf("not found: %d", entity.ID)
	}
	for _, f := range fields {
		switch f.String() {
		case "name":
			existing.Name = entity.Name
		case "status":
			existing.Status = entity.Status
		case "price":
			existing.Price = entity.Price
		}
	}
	m.data[entity.ID] = existing
	return existing, nil
}

func (m *memoryCRUD) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls++
	if _, ok := m.data[id]; !ok {
		return fmt.Errorf("not found: %d", id)
	}
	delete(m.data, id)
	return nil
}

// ── Helper: ID extractor ─────────────────────────────────────────────────

var petIDFunc = func(p Pet) int64 { return p.ID }

// ── Helper: reusable validators ──────────────────────────────────────────

// nameRequiredValidator rejects entities with an empty Name.
var nameRequiredValidator = validation.ValidatorFunc[Pet, int64](
	func(_ context.Context, req validation.Request[Pet, int64]) error {
		if req.Entity.Name == "" {
			return validation.NewError(req.Operation,
				validation.NewFieldError("name", "is required", "required"),
			)
		}
		return nil
	},
)

// pricePositiveValidator rejects entities with a non-positive Price.
var pricePositiveValidator = validation.ValidatorFunc[Pet, int64](
	func(_ context.Context, req validation.Request[Pet, int64]) error {
		if req.Entity.Price < 0 {
			return validation.NewError(req.Operation,
				validation.NewFieldError("price", "must be non-negative", "min"),
			)
		}
		return nil
	},
)

// ── Tests: Create with valid entity ──────────────────────────────────────

func TestCreate_ValidEntity_PassesThrough(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	pet, err := repo.Create(ctx, Pet{Name: "Buddy", Price: 100})
	be.NoError(t, err)
	be.RequireThat(t, pet.ID, be.NonZero())
	be.AssertThat(t, pet.Name, be.Eq("Buddy"))
	be.AssertThat(t, inner.createCalls, be.Eq(1))
}

// ── Tests: Create with invalid entity ────────────────────────────────────

func TestCreate_InvalidEntity_ReturnsValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: "", Price: 100})
	be.ErrorIs(t, err, validation.ErrValidation)
	be.AssertThat(t, inner.createCalls, be.Eq(0))
}

// ── Tests: Update with valid entity ──────────────────────────────────────

func TestUpdate_ValidEntity_PassesThrough(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	updated, err := repo.Update(ctx, Pet{ID: 1, Name: "Max", Status: "available", Price: 100})
	be.NoError(t, err)
	be.AssertThat(t, updated.Name, be.Eq("Max"))
	be.AssertThat(t, inner.updateCalls, be.Eq(1))
}

func TestUpdate_InvalidEntity_ReturnsValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	_, err := repo.Update(ctx, Pet{ID: 1, Name: "", Status: "available", Price: 100})
	be.ErrorIs(t, err, validation.ErrValidation)
	be.AssertThat(t, inner.updateCalls, be.Eq(0))
}

// ── Tests: Patch with valid entity ───────────────────────────────────────

func TestPatch_ValidEntity_PassesThrough(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	fields := r3.Fields{r3.NewFieldSpec("name")}
	patched, err := repo.Patch(ctx, Pet{ID: 1, Name: "Max"}, fields)
	be.NoError(t, err)
	be.AssertThat(t, patched.Name, be.Eq("Max"))
	be.AssertThat(t, inner.patchCalls, be.Eq(1))
}

func TestPatch_InvalidEntity_ReturnsValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	fields := r3.Fields{r3.NewFieldSpec("name")}
	_, err := repo.Patch(ctx, Pet{ID: 1, Name: ""}, fields)
	be.ErrorIs(t, err, validation.ErrValidation)
	be.AssertThat(t, inner.patchCalls, be.Eq(0))
}

// TestPatch_MergedReflectsFullEntity verifies M8: a whole-entity validator sees
// the patch merged over existing state (req.Merged), not the sparse zeroed input.
// A "name required" rule applied to a status-only patch must pass because the
// merged entity still carries the existing name.
func TestPatch_MergedReflectsFullEntity(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	var seenMerged *Pet
	mergedNameRequired := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			// Validate the post-patch state, not the sparse input.
			be.RequireThat(t, req.Merged, be.NotNil(), "expected Merged to be populated for Patch with IDFunc")
			seenMerged = req.Merged
			if req.Merged.Name == "" {
				return validation.NewError(req.Operation,
					validation.NewFieldError("name", "is required", "required"),
				)
			}
			return nil
		},
	)

	repo := validation.WithValidation[Pet, int64](inner, mergedNameRequired,
		validation.WithIDFunc[Pet, int64](petIDFunc))

	ctx := context.Background()
	// Patch only status; Name is absent from the sparse input.
	fields := r3.Fields{r3.NewFieldSpec("status")}
	_, err := repo.Patch(ctx, Pet{ID: 1, Status: "sold"}, fields)
	be.NoError(t, err, "Patch should pass - merged entity keeps Name=Buddy")

	be.AssertThat(t, seenMerged.Name, be.Eq("Buddy"))
	be.AssertThat(t, seenMerged.Status, be.Eq("sold"))
	be.AssertThat(t, inner.patchCalls, be.Eq(1))
}

// ── Tests: Patch receives Fields ─────────────────────────────────────────

func TestPatch_ValidatorReceivesFields(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	var capturedFields r3.Fields
	fieldCapture := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			capturedFields = req.Fields
			return nil
		},
	)

	repo := validation.WithValidation[Pet, int64](inner, fieldCapture)

	ctx := context.Background()
	fields := r3.Fields{r3.NewFieldSpec("name"), r3.NewFieldSpec("status")}
	_, err := repo.Patch(ctx, Pet{ID: 1, Name: "Max", Status: "sold"}, fields)
	be.NoError(t, err)
	be.RequireThat(t, capturedFields, be.HaveLength(2))
	be.AssertThat(t, capturedFields[0].String(), be.Eq("name"))
	be.AssertThat(t, capturedFields[1].String(), be.Eq("status"))
}

// ── Tests: Create does not receive Fields ────────────────────────────────

func TestCreate_ValidatorReceivesNilFields(t *testing.T) {
	inner := newMemoryCRUD()

	var capturedFields r3.Fields
	var capturedOp validation.Operation
	fieldCapture := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			capturedFields = req.Fields
			capturedOp = req.Operation
			return nil
		},
	)

	repo := validation.WithValidation[Pet, int64](inner, fieldCapture)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: "Buddy"})
	be.NoError(t, err)
	be.AssertThat(t, capturedFields, be.Nil())
	be.AssertThat(t, capturedOp, be.Eq(validation.OpCreate))
}

// ── Tests: Get, List, Delete pass through ────────────────────────────────

func TestGet_PassesThroughWithoutValidation(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy"}
	inner.nextID = 2

	// Use a validator that always fails -- Get should still work.
	alwaysFail := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, _ validation.Request[Pet, int64]) error {
			return validation.NewError(validation.OpCreate,
				validation.NewFieldError("name", "always fails", "always"),
			)
		},
	)

	repo := validation.WithValidation[Pet, int64](inner, alwaysFail)

	ctx := context.Background()
	pet, err := repo.Get(ctx, 1)
	be.NoError(t, err, "Get should pass through")
	be.AssertThat(t, pet.Name, be.Eq("Buddy"))
}

func TestList_PassesThroughWithoutValidation(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy"}
	inner.data[2] = Pet{ID: 2, Name: "Max"}
	inner.nextID = 3

	alwaysFail := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, _ validation.Request[Pet, int64]) error {
			return validation.NewError(validation.OpCreate,
				validation.NewFieldError("name", "always fails", "always"),
			)
		},
	)

	repo := validation.WithValidation[Pet, int64](inner, alwaysFail)

	ctx := context.Background()
	list, count, err := repo.List(ctx)
	be.NoError(t, err, "List should pass through")
	be.AssertThat(t, count, be.Eq(int64(2)))
	be.AssertThat(t, list, be.HaveLength(2))
}

func TestDelete_PassesThroughWithoutValidation(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy"}
	inner.nextID = 2

	alwaysFail := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, _ validation.Request[Pet, int64]) error {
			return validation.NewError(validation.OpCreate,
				validation.NewFieldError("name", "always fails", "always"),
			)
		},
	)

	repo := validation.WithValidation[Pet, int64](inner, alwaysFail)

	ctx := context.Background()
	err := repo.Delete(ctx, 1)
	be.NoError(t, err, "Delete should pass through")

	// Verify entity was actually deleted.
	_, err = inner.Get(context.Background(), 1)
	be.Error(t, err)
}

// ── Tests: WithIDFunc fetches existing entity ────────────────────────────

func TestUpdate_WithIDFunc_FetchesExisting(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	var capturedExisting *Pet
	captureExisting := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			capturedExisting = req.Existing
			return nil
		},
	)

	repo := validation.WithValidation[Pet, int64](
		inner, captureExisting,
		validation.WithIDFunc[Pet, int64](petIDFunc),
	)

	ctx := context.Background()
	_, err := repo.Update(ctx, Pet{ID: 1, Name: "Max", Status: "sold", Price: 200})
	be.NoError(t, err)
	be.RequireThat(t, capturedExisting, be.NotNil())
	be.AssertThat(t, capturedExisting.Name, be.Eq("Buddy"))
	be.AssertThat(t, capturedExisting.Status, be.Eq("available"))
}

func TestPatch_WithIDFunc_FetchesExisting(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	var capturedExisting *Pet
	captureExisting := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			capturedExisting = req.Existing
			return nil
		},
	)

	repo := validation.WithValidation[Pet, int64](
		inner, captureExisting,
		validation.WithIDFunc[Pet, int64](petIDFunc),
	)

	ctx := context.Background()
	fields := r3.Fields{r3.NewFieldSpec("status")}
	_, err := repo.Patch(ctx, Pet{ID: 1, Status: "sold"}, fields)
	be.NoError(t, err)
	be.RequireThat(t, capturedExisting, be.NotNil())
	be.AssertThat(t, capturedExisting.Status, be.Eq("available"))
}

func TestUpdate_WithoutIDFunc_ExistingIsNil(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	var capturedExisting *Pet
	captureExisting := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			capturedExisting = req.Existing
			return nil
		},
	)

	// No WithIDFunc — Existing should be nil
	repo := validation.WithValidation[Pet, int64](inner, captureExisting)

	ctx := context.Background()
	_, err := repo.Update(ctx, Pet{ID: 1, Name: "Max"})
	be.NoError(t, err)
	be.AssertThat(t, capturedExisting, be.Nil())
}

// ── Tests: State-transition validation ───────────────────────────────────

func TestUpdate_StateTransitionValidation(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "sold", Price: 100}
	inner.nextID = 2

	// Only allow status transitions: available -> pending -> sold. No going back.
	statusValidator := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			if req.Existing == nil {
				return nil // can't validate transitions without existing state
			}
			oldStatus := req.Existing.Status
			newStatus := req.Entity.Status
			if oldStatus == "sold" && newStatus != "sold" {
				return validation.NewError(req.Operation,
					validation.NewFieldError("status", "cannot change status from sold", "status_transition"),
				)
			}
			return nil
		},
	)

	repo := validation.WithValidation[Pet, int64](
		inner, statusValidator,
		validation.WithIDFunc[Pet, int64](petIDFunc),
	)

	ctx := context.Background()

	// Try to change a sold pet back to available — should fail.
	_, err := repo.Update(ctx, Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100})
	be.ErrorIs(t, err, validation.ErrValidation)
	be.AssertThat(t, inner.updateCalls, be.Eq(0))

	// Check the field error
	var ve *validation.Error
	be.RequireThat(t, errors.As(err, &ve), be.True())
	be.AssertThat(t, ve.HasField("status"), be.True())
}

// ── Tests: NoValidation helper ───────────────────────────────────────────

func TestNoValidation_AllowsEverything(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, validation.NoValidation[Pet, int64]())

	ctx := context.Background()

	// Create
	pet, err := repo.Create(ctx, Pet{Name: ""}) // empty name should be fine with NoValidation
	be.NoError(t, err)

	// Update
	pet.Name = "Updated"
	_, err = repo.Update(ctx, pet)
	be.NoError(t, err)

	// Patch
	_, err = repo.Patch(ctx, pet, r3.Fields{r3.NewFieldSpec("name")})
	be.NoError(t, err)
}

// ── Tests: Compose helper ────────────────────────────────────────────────

func TestCompose_CollectsAllErrors(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](
		inner,
		validation.Compose[Pet, int64](nameRequiredValidator, pricePositiveValidator),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: "", Price: -10})
	be.ErrorIs(t, err, validation.ErrValidation)

	var ve *validation.Error
	be.RequireThat(t, errors.As(err, &ve), be.True())
	be.RequireThat(t, ve.Errors, be.HaveLength(2))
	be.AssertThat(t, ve.HasField("name"), be.True())
	be.AssertThat(t, ve.HasField("price"), be.True())
}

func TestCompose_NoErrors(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](
		inner,
		validation.Compose[Pet, int64](nameRequiredValidator, pricePositiveValidator),
	)

	ctx := context.Background()
	pet, err := repo.Create(ctx, Pet{Name: "Buddy", Price: 100})
	be.NoError(t, err)
	be.AssertThat(t, pet.Name, be.Eq("Buddy"))
}

func TestCompose_Empty_AllowsEverything(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](
		inner,
		validation.Compose[Pet, int64](),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: ""})
	be.NoError(t, err, "Create with empty Compose failed")
}

func TestCompose_ShortCircuitsOnNonValidationError(t *testing.T) {
	inner := newMemoryCRUD()

	fatalError := errors.New("database connection lost")
	fatalValidator := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, _ validation.Request[Pet, int64]) error {
			return fatalError
		},
	)

	repo := validation.WithValidation[Pet, int64](
		inner,
		validation.Compose[Pet, int64](fatalValidator, nameRequiredValidator),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: "Buddy"})
	be.ErrorIs(t, err, fatalError)
	// Should NOT be a validation error.
	be.RequireThat(t, err, be.Not(be.MatchError(validation.ErrValidation)),
		"non-validation errors should not be wrapped as ErrValidation")
}

// ── Tests: OperationValidators helper ────────────────────────────────────

func TestOperationValidators_RoutesCorrectly(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	// Create requires name, but Update/Patch don't.
	repo := validation.WithValidation[Pet, int64](
		inner,

		validation.OperationValidators[Pet, int64](map[validation.Operation]validation.Validator[Pet, int64]{
			validation.OpCreate: nameRequiredValidator,
		}),
	)

	ctx := context.Background()

	// Create without name should fail.
	_, err := repo.Create(ctx, Pet{Name: ""})
	be.ErrorIs(t, err, validation.ErrValidation)

	// Update without name should pass (OpUpdate not mapped).
	_, err = repo.Update(ctx, Pet{ID: 1, Name: ""})
	be.NoError(t, err, "Update should pass for unmapped operation")

	// Patch should also pass.
	_, err = repo.Patch(ctx, Pet{ID: 1, Name: ""}, r3.Fields{r3.NewFieldSpec("name")})
	be.NoError(t, err, "Patch should pass for unmapped operation")
}

// ── Tests: ValidationError ───────────────────────────────────────────────

func TestValidationError_Is(t *testing.T) {
	err := validation.NewError(validation.OpCreate,
		validation.NewFieldError("name", "is required", "required"),
	)

	be.ErrorIs(t, err, validation.ErrValidation,
		"expected ValidationError to satisfy errors.Is(err, ErrValidation)")
}

func TestValidationError_ErrorMessage(t *testing.T) {
	err := validation.NewError(validation.OpCreate,
		validation.NewFieldError("name", "is required", "required"),
		validation.NewFieldError("price", "must be positive", "min"),
	)

	msg := err.Error()
	expected := "r3/validation: validation failed on create: name: is required; price: must be positive"
	be.AssertThat(t, msg, be.Eq(expected))
}

func TestValidationError_ErrorMessage_NoFields(t *testing.T) {
	err := validation.NewError(validation.OpUpdate)
	msg := err.Error()
	expected := "r3/validation: validation failed on update"
	be.AssertThat(t, msg, be.Eq(expected))
}

func TestValidationError_HasField(t *testing.T) {
	err := validation.NewError(validation.OpCreate,
		validation.NewFieldError("name", "is required", "required"),
		validation.NewFieldError("price", "must be positive", "min"),
	)

	be.AssertThat(t, err.HasField("name"), be.True())
	be.AssertThat(t, err.HasField("price"), be.True())
	be.AssertThat(t, err.HasField("status"), be.False())
}

func TestFieldError_Error(t *testing.T) {
	fe := validation.NewFieldError("name", "is required", "required")
	be.AssertThat(t, fe.Error(), be.Eq("name: is required"))

	fe2 := validation.NewFieldError("", "general error", "general")
	be.AssertThat(t, fe2.Error(), be.Eq("general error"))
}

// ── Tests: ValidatorFunc adapter ─────────────────────────────────────────

func TestValidatorFunc(t *testing.T) {
	called := false
	v := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, _ validation.Request[Pet, int64]) error {
			called = true
			return nil
		},
	)

	err := v.Validate(context.Background(), validation.Request[Pet, int64]{})
	be.NoError(t, err)
	be.RequireThat(t, called, be.True())
}

// ── Tests: Inner() ───────────────────────────────────────────────────────

func TestInner(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, validation.NoValidation[Pet, int64]())

	got := repo.Inner()
	be.AssertThat(t, got, be.Identical(inner))
}

// ── Tests: Update/Patch with IDFunc and entity not found ─────────────────

func TestUpdate_WithIDFunc_EntityNotFound(t *testing.T) {
	inner := newMemoryCRUD()
	// No data seeded — entity does not exist.

	repo := validation.WithValidation[Pet, int64](
		inner, nameRequiredValidator,
		validation.WithIDFunc[Pet, int64](petIDFunc),
	)

	ctx := context.Background()
	_, err := repo.Update(ctx, Pet{ID: 999, Name: "Ghost"})
	be.Error(t, err)
	// Should be a "not found" error from Get, not a validation error.
	be.RequireThat(t, err, be.Not(be.MatchError(validation.ErrValidation)),
		"expected non-validation error for missing entity")
}

func TestPatch_WithIDFunc_EntityNotFound(t *testing.T) {
	inner := newMemoryCRUD()

	repo := validation.WithValidation[Pet, int64](
		inner, nameRequiredValidator,
		validation.WithIDFunc[Pet, int64](petIDFunc),
	)

	ctx := context.Background()
	_, err := repo.Patch(ctx, Pet{ID: 999, Name: "Ghost"}, r3.Fields{r3.NewFieldSpec("name")})
	be.Error(t, err)
	be.RequireThat(t, err, be.Not(be.MatchError(validation.ErrValidation)),
		"expected non-validation error for missing entity")
}

// ── Tests: Operation values in ValidationRequest ─────────────────────────

func TestOperationValues(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	var capturedOps []validation.Operation
	captureOp := validation.ValidatorFunc[Pet, int64](
		func(_ context.Context, req validation.Request[Pet, int64]) error {
			capturedOps = append(capturedOps, req.Operation)
			return nil
		},
	)

	repo := validation.WithValidation[Pet, int64](inner, captureOp)
	ctx := context.Background()

	// Create
	created, _ := repo.Create(ctx, Pet{Name: "New"})

	// Update
	created.Name = "Updated"
	_, _ = repo.Update(ctx, created)

	// Patch
	_, _ = repo.Patch(ctx, created, r3.Fields{r3.NewFieldSpec("name")})

	be.RequireThat(t, capturedOps, be.HaveLength(3))
	be.AssertThat(t, capturedOps[0], be.Eq(validation.OpCreate))
	be.AssertThat(t, capturedOps[1], be.Eq(validation.OpUpdate))
	be.AssertThat(t, capturedOps[2], be.Eq(validation.OpPatch))
}

// ── Tests: errors.As works with ValidationError ──────────────────────────

func TestErrorsAs_ValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: ""})

	var ve *validation.Error
	be.RequireThat(t, errors.As(err, &ve), be.True(),
		"errors.As should work with *ValidationError")
	be.AssertThat(t, ve.Operation, be.Eq(validation.OpCreate))
	be.RequireThat(t, ve.Errors, be.HaveLength(1))
	be.AssertThat(t, ve.Errors[0].Field, be.Eq("name"))
	be.AssertThat(t, ve.Errors[0].Code, be.Eq("required"))
}
