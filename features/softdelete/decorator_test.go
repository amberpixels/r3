package softdelete_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/softdelete"
)

// ── Test entity ──────────────────────────────────────────────────────────

type User struct {
	ID    int64  `db:"id,pk" json:"id"`
	Name  string `db:"name"  json:"name"`
	Email string `db:"email" json:"email"`
}

// ── In-memory CRUD mock ──────────────────────────────────────────────────

type memoryCRUD struct {
	mu      sync.Mutex
	data    map[int64]User
	deleted map[int64]User // soft-deleted records
	nextID  int64
}

func newMemoryCRUD() *memoryCRUD {
	return &memoryCRUD{
		data:    make(map[int64]User),
		deleted: make(map[int64]User),
		nextID:  1,
	}
}

func (m *memoryCRUD) Create(_ context.Context, entity User) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity.ID = m.nextID
	m.nextID++
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Get(_ context.Context, id int64, _ ...r3.Query) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity, ok := m.data[id]
	if !ok {
		return User{}, fmt.Errorf("not found: %d", id)
	}
	return entity, nil
}

func (m *memoryCRUD) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, qarg...)
	return n, err
}

func (m *memoryCRUD) List(_ context.Context, _ ...r3.Query) ([]User, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []User
	for _, v := range m.data {
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *memoryCRUD) Update(_ context.Context, entity User) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[entity.ID]; !ok {
		return User{}, fmt.Errorf("not found: %d", entity.ID)
	}
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Patch(_ context.Context, entity User, fields r3.Fields) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.data[entity.ID]
	if !ok {
		return User{}, fmt.Errorf("not found: %d", entity.ID)
	}
	for _, f := range fields {
		switch f.String() {
		case "name":
			existing.Name = entity.Name
		case "email":
			existing.Email = entity.Email
		}
	}
	m.data[entity.ID] = existing
	return existing, nil
}

func (m *memoryCRUD) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity, ok := m.data[id]
	if !ok {
		return fmt.Errorf("not found: %d", id)
	}
	// Soft-delete: move to deleted map
	delete(m.data, id)
	m.deleted[id] = entity
	return nil
}

// Restore implements softdelete.SoftDeleter.
func (m *memoryCRUD) Restore(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity, ok := m.deleted[id]
	if !ok {
		return fmt.Errorf("not found in deleted: %d", id)
	}
	delete(m.deleted, id)
	m.data[id] = entity
	return nil
}

// HardDelete implements softdelete.SoftDeleter.
func (m *memoryCRUD) HardDelete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[id]; ok {
		delete(m.data, id)
		return nil
	}
	if _, ok := m.deleted[id]; ok {
		delete(m.deleted, id)
		return nil
	}
	return fmt.Errorf("not found: %d", id)
}

// ── In-memory CRUD mock WITHOUT SoftDeleter ──────────────────────────────

type plainCRUD struct {
	mu     sync.Mutex
	data   map[int64]User
	nextID int64
}

func newPlainCRUD() *plainCRUD {
	return &plainCRUD{data: make(map[int64]User), nextID: 1}
}

func (m *plainCRUD) Create(_ context.Context, entity User) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity.ID = m.nextID
	m.nextID++
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *plainCRUD) Get(_ context.Context, id int64, _ ...r3.Query) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entity, ok := m.data[id]
	if !ok {
		return User{}, fmt.Errorf("not found: %d", id)
	}
	return entity, nil
}

func (m *plainCRUD) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, qarg...)
	return n, err
}

func (m *plainCRUD) List(_ context.Context, _ ...r3.Query) ([]User, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []User
	for _, v := range m.data {
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *plainCRUD) Update(_ context.Context, entity User) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *plainCRUD) Patch(_ context.Context, entity User, _ r3.Fields) (User, error) {
	return entity, nil
}

func (m *plainCRUD) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id)
	return nil
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestCRUD_PassthroughCreate(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, err := repo.Create(ctx, User{Name: "Alice", Email: "alice@example.com"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if user.Name != "Alice" {
		t.Errorf("expected Name='Alice', got %q", user.Name)
	}
}

func TestCRUD_PassthroughGet(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	created, _ := repo.Create(ctx, User{Name: "Bob", Email: "bob@example.com"})

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "Bob" {
		t.Errorf("expected Name='Bob', got %q", got.Name)
	}
}

func TestCRUD_PassthroughList(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	_, _ = repo.Create(ctx, User{Name: "A"})
	_, _ = repo.Create(ctx, User{Name: "B"})

	list, count, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2, got %d", count)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 items, got %d", len(list))
	}
}

func TestCRUD_PassthroughUpdate(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Carol"})
	user.Name = "Carol Updated"
	updated, err := repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "Carol Updated" {
		t.Errorf("expected Name='Carol Updated', got %q", updated.Name)
	}
}

func TestCRUD_PassthroughPatch(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Dave", Email: "dave@example.com"})
	user.Email = "newemail@example.com"
	patched, err := repo.Patch(ctx, user, r3.Fields{r3.NewFieldSpec("email")})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if patched.Email != "newemail@example.com" {
		t.Errorf("expected Email='newemail@example.com', got %q", patched.Email)
	}
}

func TestCRUD_PassthroughDelete(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Eve"})

	err := repo.Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should be soft-deleted (moved to deleted map in mock)
	_, err = repo.Get(ctx, user.ID)
	if err == nil {
		t.Fatal("expected Get to fail after soft-delete")
	}
}

// passthroughDecorator is a minimal decorator that forwards every CRUD method
// and participates in the Unwrap chain, but does NOT implement SoftDeleter. It
// simulates another feature (history, validation, ...) sitting between the
// soft-delete decorator and the backend.
type passthroughDecorator struct {
	inner r3.CRUD[User, int64]
}

func (d *passthroughDecorator) Create(ctx context.Context, e User) (User, error) {
	return d.inner.Create(ctx, e)
}
func (d *passthroughDecorator) Get(ctx context.Context, id int64, q ...r3.Query) (User, error) {
	return d.inner.Get(ctx, id, q...)
}

func (d *passthroughDecorator) List(ctx context.Context, q ...r3.Query) ([]User, int64, error) {
	return d.inner.List(ctx, q...)
}
func (d *passthroughDecorator) Count(ctx context.Context, q ...r3.Query) (int64, error) {
	return d.inner.Count(ctx, q...)
}
func (d *passthroughDecorator) Update(ctx context.Context, e User) (User, error) {
	return d.inner.Update(ctx, e)
}

func (d *passthroughDecorator) Patch(ctx context.Context, e User, f r3.Fields) (User, error) {
	return d.inner.Patch(ctx, e, f)
}
func (d *passthroughDecorator) Delete(ctx context.Context, id int64) error {
	return d.inner.Delete(ctx, id)
}
func (d *passthroughDecorator) Unwrap() r3.CRUD[User, int64] { return d.inner }

// TestCRUD_RestoreThroughIntermediateDecorator verifies that Restore reaches a
// backend SoftDeleter even when another (non-soft-delete) decorator sits
// between the soft-delete decorator and the backend (H9). Before the fix, the
// non-recursive type assertion returned ErrNotSoftDeletable here.
func TestCRUD_RestoreThroughIntermediateDecorator(t *testing.T) {
	backend := newMemoryCRUD()
	middle := &passthroughDecorator{inner: backend}
	repo := softdelete.WithSoftDelete[User, int64](middle)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Heidi"})
	if err := repo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if err := repo.Restore(ctx, user.ID); err != nil {
		t.Fatalf("Restore through intermediate decorator failed: %v", err)
	}
	restored, err := repo.Get(ctx, user.ID)
	if err != nil {
		t.Fatalf("Get after Restore failed: %v", err)
	}
	if restored.Name != "Heidi" {
		t.Errorf("expected Name='Heidi', got %q", restored.Name)
	}

	if err := repo.HardDelete(ctx, user.ID); err != nil {
		t.Fatalf("HardDelete through intermediate decorator failed: %v", err)
	}
}

func TestCRUD_RestoreDelegatesToSoftDeleter(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Frank"})

	// Soft-delete
	_ = repo.Delete(ctx, user.ID)

	// Verify it's gone from active
	_, err := repo.Get(ctx, user.ID)
	if err == nil {
		t.Fatal("expected user to be soft-deleted")
	}

	// Restore
	err = repo.Restore(ctx, user.ID)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Should be back
	restored, err := repo.Get(ctx, user.ID)
	if err != nil {
		t.Fatalf("Get after Restore failed: %v", err)
	}
	if restored.Name != "Frank" {
		t.Errorf("expected Name='Frank', got %q", restored.Name)
	}
}

func TestCRUD_HardDeleteDelegatesToSoftDeleter(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Grace"})

	// HardDelete should permanently remove
	err := repo.HardDelete(ctx, user.ID)
	if err != nil {
		t.Fatalf("HardDelete failed: %v", err)
	}

	// Should not be in active or deleted
	inner.mu.Lock()
	_, inActive := inner.data[user.ID]
	_, inDeleted := inner.deleted[user.ID]
	inner.mu.Unlock()

	if inActive {
		t.Error("expected user not to be in active data after HardDelete")
	}
	if inDeleted {
		t.Error("expected user not to be in deleted data after HardDelete")
	}
}

func TestCRUD_HardDeleteSoftDeletedRecord(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Heidi"})

	// Soft-delete first
	_ = repo.Delete(ctx, user.ID)

	// Then hard-delete the soft-deleted record
	err := repo.HardDelete(ctx, user.ID)
	if err != nil {
		t.Fatalf("HardDelete of soft-deleted record failed: %v", err)
	}

	// Should be completely gone
	inner.mu.Lock()
	_, inActive := inner.data[user.ID]
	_, inDeleted := inner.deleted[user.ID]
	inner.mu.Unlock()

	if inActive || inDeleted {
		t.Error("expected user to be permanently removed after HardDelete")
	}
}

func TestCRUD_RestoreReturnsErrNotSoftDeletable(t *testing.T) {
	inner := newPlainCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Ivan"})

	err := repo.Restore(ctx, user.ID)
	if !errors.Is(err, softdelete.ErrNotSoftDeletable) {
		t.Fatalf("expected ErrNotSoftDeletable, got %v", err)
	}
}

func TestCRUD_HardDeleteReturnsErrNotSoftDeletable(t *testing.T) {
	inner := newPlainCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Judy"})

	err := repo.HardDelete(ctx, user.ID)
	if !errors.Is(err, softdelete.ErrNotSoftDeletable) {
		t.Fatalf("expected ErrNotSoftDeletable, got %v", err)
	}
}

func TestCRUD_Inner(t *testing.T) {
	inner := newMemoryCRUD()
	repo := softdelete.WithSoftDelete[User, int64](inner)

	// Inner() should return the original inner CRUD
	got := repo.Inner()
	if got != inner {
		t.Error("Inner() should return the wrapped CRUD")
	}
}
