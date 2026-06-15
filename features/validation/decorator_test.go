package validation_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

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
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if pet.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if pet.Name != "Buddy" {
		t.Errorf("expected Name='Buddy', got %q", pet.Name)
	}
	if inner.createCalls != 1 {
		t.Errorf("expected 1 inner.Create call, got %d", inner.createCalls)
	}
}

// ── Tests: Create with invalid entity ────────────────────────────────────

func TestCreate_InvalidEntity_ReturnsValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: "", Price: 100})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, validation.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if inner.createCalls != 0 {
		t.Error("inner.Create should not have been called")
	}
}

// ── Tests: Update with valid entity ──────────────────────────────────────

func TestUpdate_ValidEntity_PassesThrough(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	updated, err := repo.Update(ctx, Pet{ID: 1, Name: "Max", Status: "available", Price: 100})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "Max" {
		t.Errorf("expected Name='Max', got %q", updated.Name)
	}
	if inner.updateCalls != 1 {
		t.Errorf("expected 1 inner.Update call, got %d", inner.updateCalls)
	}
}

func TestUpdate_InvalidEntity_ReturnsValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	_, err := repo.Update(ctx, Pet{ID: 1, Name: "", Status: "available", Price: 100})
	if !errors.Is(err, validation.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if inner.updateCalls != 0 {
		t.Error("inner.Update should not have been called")
	}
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
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if patched.Name != "Max" {
		t.Errorf("expected Name='Max', got %q", patched.Name)
	}
	if inner.patchCalls != 1 {
		t.Errorf("expected 1 inner.Patch call, got %d", inner.patchCalls)
	}
}

func TestPatch_InvalidEntity_ReturnsValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	fields := r3.Fields{r3.NewFieldSpec("name")}
	_, err := repo.Patch(ctx, Pet{ID: 1, Name: ""}, fields)
	if !errors.Is(err, validation.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if inner.patchCalls != 0 {
		t.Error("inner.Patch should not have been called")
	}
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
			if req.Merged == nil {
				t.Fatal("expected Merged to be populated for Patch with IDFunc")
			}
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
	if err != nil {
		t.Fatalf("Patch should pass — merged entity keeps Name=Buddy: %v", err)
	}

	if seenMerged.Name != "Buddy" {
		t.Errorf("Merged.Name = %q, want preserved %q", seenMerged.Name, "Buddy")
	}
	if seenMerged.Status != "sold" {
		t.Errorf("Merged.Status = %q, want patched %q", seenMerged.Status, "sold")
	}
	if inner.patchCalls != 1 {
		t.Errorf("expected 1 inner.Patch call, got %d", inner.patchCalls)
	}
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
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if len(capturedFields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(capturedFields))
	}
	if capturedFields[0].String() != "name" {
		t.Errorf("expected first field='name', got %q", capturedFields[0].String())
	}
	if capturedFields[1].String() != "status" {
		t.Errorf("expected second field='status', got %q", capturedFields[1].String())
	}
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
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if capturedFields != nil {
		t.Errorf("expected nil Fields for Create, got %v", capturedFields)
	}
	if capturedOp != validation.OpCreate {
		t.Errorf("expected OpCreate, got %q", capturedOp)
	}
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
	if err != nil {
		t.Fatalf("Get should pass through: %v", err)
	}
	if pet.Name != "Buddy" {
		t.Errorf("expected Name='Buddy', got %q", pet.Name)
	}
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
	if err != nil {
		t.Fatalf("List should pass through: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 items, got %d", len(list))
	}
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
	if err := repo.Delete(ctx, 1); err != nil {
		t.Fatalf("Delete should pass through: %v", err)
	}

	// Verify entity was actually deleted.
	_, err := inner.Get(context.Background(), 1)
	if err == nil {
		t.Fatal("expected entity to be deleted")
	}
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
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if capturedExisting == nil {
		t.Fatal("expected Existing to be populated")
	}
	if capturedExisting.Name != "Buddy" {
		t.Errorf("expected Existing.Name='Buddy', got %q", capturedExisting.Name)
	}
	if capturedExisting.Status != "available" {
		t.Errorf("expected Existing.Status='available', got %q", capturedExisting.Status)
	}
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
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if capturedExisting == nil {
		t.Fatal("expected Existing to be populated")
	}
	if capturedExisting.Status != "available" {
		t.Errorf("expected Existing.Status='available', got %q", capturedExisting.Status)
	}
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
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if capturedExisting != nil {
		t.Error("expected Existing to be nil without IDFunc")
	}
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
	if !errors.Is(err, validation.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
	if inner.updateCalls != 0 {
		t.Error("inner.Update should not have been called")
	}

	// Check the field error
	var ve *validation.Error
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if !ve.HasField("status") {
		t.Error("expected field error for 'status'")
	}
}

// ── Tests: NoValidation helper ───────────────────────────────────────────

func TestNoValidation_AllowsEverything(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, validation.NoValidation[Pet, int64]())

	ctx := context.Background()

	// Create
	pet, err := repo.Create(ctx, Pet{Name: ""}) // empty name should be fine with NoValidation
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	pet.Name = "Updated"
	_, err = repo.Update(ctx, pet)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Patch
	_, err = repo.Patch(ctx, pet, r3.Fields{r3.NewFieldSpec("name")})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
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
	if !errors.Is(err, validation.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}

	var ve *validation.Error
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Errors) != 2 {
		t.Fatalf("expected 2 field errors, got %d", len(ve.Errors))
	}
	if !ve.HasField("name") {
		t.Error("expected field error for 'name'")
	}
	if !ve.HasField("price") {
		t.Error("expected field error for 'price'")
	}
}

func TestCompose_NoErrors(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](
		inner,
		validation.Compose[Pet, int64](nameRequiredValidator, pricePositiveValidator),
	)

	ctx := context.Background()
	pet, err := repo.Create(ctx, Pet{Name: "Buddy", Price: 100})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if pet.Name != "Buddy" {
		t.Errorf("expected Name='Buddy', got %q", pet.Name)
	}
}

func TestCompose_Empty_AllowsEverything(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](
		inner,
		validation.Compose[Pet, int64](),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: ""})
	if err != nil {
		t.Fatalf("Create with empty Compose failed: %v", err)
	}
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
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fatalError) {
		t.Fatalf("expected fatalError, got %v", err)
	}
	// Should NOT be a validation error.
	if errors.Is(err, validation.ErrValidation) {
		t.Fatal("non-validation errors should not be wrapped as ErrValidation")
	}
}

// ── Tests: OperationValidators helper ────────────────────────────────────

func TestOperationValidators_RoutesCorrectly(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Pet{ID: 1, Name: "Buddy", Status: "available", Price: 100}
	inner.nextID = 2

	// Create requires name, but Update/Patch don't.
	repo := validation.WithValidation[Pet, int64](
		inner,
		//nolint:exhaustive // intentionally partial map; unmapped ops pass through
		validation.OperationValidators[Pet, int64](map[validation.Operation]validation.Validator[Pet, int64]{
			validation.OpCreate: nameRequiredValidator,
		}),
	)

	ctx := context.Background()

	// Create without name should fail.
	_, err := repo.Create(ctx, Pet{Name: ""})
	if !errors.Is(err, validation.ErrValidation) {
		t.Fatalf("Create: expected ErrValidation, got %v", err)
	}

	// Update without name should pass (OpUpdate not mapped).
	_, err = repo.Update(ctx, Pet{ID: 1, Name: ""})
	if err != nil {
		t.Fatalf("Update should pass for unmapped operation: %v", err)
	}

	// Patch should also pass.
	_, err = repo.Patch(ctx, Pet{ID: 1, Name: ""}, r3.Fields{r3.NewFieldSpec("name")})
	if err != nil {
		t.Fatalf("Patch should pass for unmapped operation: %v", err)
	}
}

// ── Tests: ValidationError ───────────────────────────────────────────────

func TestValidationError_Is(t *testing.T) {
	err := validation.NewError(validation.OpCreate,
		validation.NewFieldError("name", "is required", "required"),
	)

	if !errors.Is(err, validation.ErrValidation) {
		t.Fatal("expected ValidationError to satisfy errors.Is(err, ErrValidation)")
	}
}

func TestValidationError_ErrorMessage(t *testing.T) {
	err := validation.NewError(validation.OpCreate,
		validation.NewFieldError("name", "is required", "required"),
		validation.NewFieldError("price", "must be positive", "min"),
	)

	msg := err.Error()
	expected := "r3/validation: validation failed on create: name: is required; price: must be positive"
	if msg != expected {
		t.Errorf("error message mismatch:\n  got:  %q\n  want: %q", msg, expected)
	}
}

func TestValidationError_ErrorMessage_NoFields(t *testing.T) {
	err := validation.NewError(validation.OpUpdate)
	msg := err.Error()
	expected := "r3/validation: validation failed on update"
	if msg != expected {
		t.Errorf("error message mismatch:\n  got:  %q\n  want: %q", msg, expected)
	}
}

func TestValidationError_HasField(t *testing.T) {
	err := validation.NewError(validation.OpCreate,
		validation.NewFieldError("name", "is required", "required"),
		validation.NewFieldError("price", "must be positive", "min"),
	)

	if !err.HasField("name") {
		t.Error("expected HasField('name') to be true")
	}
	if !err.HasField("price") {
		t.Error("expected HasField('price') to be true")
	}
	if err.HasField("status") {
		t.Error("expected HasField('status') to be false")
	}
}

func TestFieldError_Error(t *testing.T) {
	fe := validation.NewFieldError("name", "is required", "required")
	if fe.Error() != "name: is required" {
		t.Errorf("expected 'name: is required', got %q", fe.Error())
	}

	fe2 := validation.NewFieldError("", "general error", "general")
	if fe2.Error() != "general error" {
		t.Errorf("expected 'general error', got %q", fe2.Error())
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("ValidatorFunc was not called")
	}
}

// ── Tests: Inner() ───────────────────────────────────────────────────────

func TestInner(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, validation.NoValidation[Pet, int64]())

	got := repo.Inner()
	if got != inner {
		t.Error("Inner() should return the wrapped CRUD")
	}
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
	if err == nil {
		t.Fatal("expected error when entity not found")
	}
	// Should be a "not found" error from Get, not a validation error.
	if errors.Is(err, validation.ErrValidation) {
		t.Fatal("expected non-validation error for missing entity")
	}
}

func TestPatch_WithIDFunc_EntityNotFound(t *testing.T) {
	inner := newMemoryCRUD()

	repo := validation.WithValidation[Pet, int64](
		inner, nameRequiredValidator,
		validation.WithIDFunc[Pet, int64](petIDFunc),
	)

	ctx := context.Background()
	_, err := repo.Patch(ctx, Pet{ID: 999, Name: "Ghost"}, r3.Fields{r3.NewFieldSpec("name")})
	if err == nil {
		t.Fatal("expected error when entity not found")
	}
	if errors.Is(err, validation.ErrValidation) {
		t.Fatal("expected non-validation error for missing entity")
	}
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

	if len(capturedOps) != 3 {
		t.Fatalf("expected 3 operations, got %d", len(capturedOps))
	}
	if capturedOps[0] != validation.OpCreate {
		t.Errorf("expected OpCreate, got %q", capturedOps[0])
	}
	if capturedOps[1] != validation.OpUpdate {
		t.Errorf("expected OpUpdate, got %q", capturedOps[1])
	}
	if capturedOps[2] != validation.OpPatch {
		t.Errorf("expected OpPatch, got %q", capturedOps[2])
	}
}

// ── Tests: errors.As works with ValidationError ──────────────────────────

func TestErrorsAs_ValidationError(t *testing.T) {
	inner := newMemoryCRUD()
	repo := validation.WithValidation[Pet, int64](inner, nameRequiredValidator)

	ctx := context.Background()
	_, err := repo.Create(ctx, Pet{Name: ""})

	var ve *validation.Error
	if !errors.As(err, &ve) {
		t.Fatalf("errors.As should work with *ValidationError, got %T", err)
	}
	if ve.Operation != validation.OpCreate {
		t.Errorf("expected Operation='create', got %q", ve.Operation)
	}
	if len(ve.Errors) != 1 {
		t.Fatalf("expected 1 field error, got %d", len(ve.Errors))
	}
	if ve.Errors[0].Field != "name" {
		t.Errorf("expected field 'name', got %q", ve.Errors[0].Field)
	}
	if ve.Errors[0].Code != "required" {
		t.Errorf("expected code 'required', got %q", ve.Errors[0].Code)
	}
}
