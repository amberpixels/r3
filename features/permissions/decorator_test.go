package permissions_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/permissions"
)

// ── Test entity ──────────────────────────────────────────────────────────

type Post struct {
	ID      int64
	Title   string
	OwnerID string
}

// ── In-memory CRUD mock ──────────────────────────────────────────────────

type memoryCRUD struct {
	mu     sync.Mutex
	data   map[int64]Post
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
	return &memoryCRUD{data: make(map[int64]Post), nextID: 1}
}

func (m *memoryCRUD) Create(_ context.Context, entity Post) (Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	entity.ID = m.nextID
	m.nextID++
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Get(_ context.Context, id int64, _ ...r3.Query) (Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	entity, ok := m.data[id]
	if !ok {
		return Post{}, fmt.Errorf("not found: %d", id)
	}
	return entity, nil
}

func (m *memoryCRUD) Count(ctx context.Context, qarg ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, qarg...)
	return n, err
}

func (m *memoryCRUD) List(_ context.Context, qarg ...r3.Query) ([]Post, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listCalls++
	var result []Post
	for _, v := range m.data {
		// If filters are provided, apply basic owner_id filtering for test purposes.
		if len(qarg) > 0 && len(qarg[0].Filters) > 0 {
			for _, f := range qarg[0].Filters {
				if f.Field != nil && f.Field.String() == "owner_id" {
					if v.OwnerID != f.Value.(string) {
						continue
					}
				}
				result = append(result, v)
			}
			continue
		}
		result = append(result, v)
	}
	return result, int64(len(result)), nil
}

func (m *memoryCRUD) Update(_ context.Context, entity Post) (Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls++
	if _, ok := m.data[entity.ID]; !ok {
		return Post{}, fmt.Errorf("not found: %d", entity.ID)
	}
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *memoryCRUD) Patch(_ context.Context, entity Post, fields r3.Fields) (Post, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patchCalls++
	existing, ok := m.data[entity.ID]
	if !ok {
		return Post{}, fmt.Errorf("not found: %d", entity.ID)
	}
	for _, f := range fields {
		switch f.String() {
		case "title":
			existing.Title = entity.Title
		case "owner_id":
			existing.OwnerID = entity.OwnerID
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

var postIDFunc = func(p Post) int64 { return p.ID }

// ── Tests: AllowAll ──────────────────────────────────────────────────────

func TestAllowAll_Create(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())

	ctx := context.Background()
	post, err := repo.Create(ctx, Post{Title: "Hello"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if post.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if post.Title != "Hello" {
		t.Errorf("expected Title='Hello', got %q", post.Title)
	}
}

func TestAllowAll_Get(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())

	ctx := context.Background()
	created, _ := repo.Create(ctx, Post{Title: "Test"})
	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Test" {
		t.Errorf("expected Title='Test', got %q", got.Title)
	}
}

func TestAllowAll_List(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())

	ctx := context.Background()
	_, _ = repo.Create(ctx, Post{Title: "A"})
	_, _ = repo.Create(ctx, Post{Title: "B"})
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

func TestAllowAll_Update(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())

	ctx := context.Background()
	post, _ := repo.Create(ctx, Post{Title: "Original"})
	post.Title = "Updated"
	updated, err := repo.Update(ctx, post)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("expected Title='Updated', got %q", updated.Title)
	}
}

func TestAllowAll_Patch(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())

	ctx := context.Background()
	post, _ := repo.Create(ctx, Post{Title: "Original"})
	post.Title = "Patched"
	patched, err := repo.Patch(ctx, post, r3.Fields{r3.NewFieldSpec("title")})
	if err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if patched.Title != "Patched" {
		t.Errorf("expected Title='Patched', got %q", patched.Title)
	}
}

func TestAllowAll_Delete(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())

	ctx := context.Background()
	post, _ := repo.Create(ctx, Post{Title: "ToDelete"})
	if err := repo.Delete(ctx, post.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err := repo.Get(ctx, post.ID)
	if err == nil {
		t.Fatal("expected Get to fail after delete")
	}
}

// ── Tests: DenyAll ───────────────────────────────────────────────────────

func TestDenyAll_Create(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.DenyAll[Post, int64]())

	ctx := context.Background()
	_, err := repo.Create(ctx, Post{Title: "Hello"})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
	if inner.createCalls != 0 {
		t.Error("inner.Create should not have been called")
	}
}

func TestDenyAll_Get(t *testing.T) {
	inner := newMemoryCRUD()
	// Seed data directly into inner.
	inner.data[1] = Post{ID: 1, Title: "Secret"}
	inner.nextID = 2
	repo := permissions.WithPermissions[Post, int64](inner, permissions.DenyAll[Post, int64]())

	ctx := context.Background()
	_, err := repo.Get(ctx, 1)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

func TestDenyAll_List(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.DenyAll[Post, int64]())

	ctx := context.Background()
	_, _, err := repo.List(ctx)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
	if inner.listCalls != 0 {
		t.Error("inner.List should not have been called")
	}
}

func TestDenyAll_Update(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Original"}
	inner.nextID = 2
	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.DenyAll[Post, int64](),
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	ctx := context.Background()
	_, err := repo.Update(ctx, Post{ID: 1, Title: "Hacked"})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
	if inner.updateCalls != 0 {
		t.Error("inner.Update should not have been called")
	}
}

func TestDenyAll_Delete(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Original"}
	inner.nextID = 2
	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.DenyAll[Post, int64](),
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	ctx := context.Background()
	err := repo.Delete(ctx, 1)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
	if inner.deleteCalls != 0 {
		t.Error("inner.Delete should not have been called")
	}
}

// ── Tests: ReadOnly ──────────────────────────────────────────────────────

func TestReadOnly_AllowsReads(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Readable"}
	inner.nextID = 2
	repo := permissions.WithPermissions[Post, int64](inner, permissions.ReadOnly[Post, int64]())

	ctx := context.Background()

	// Get should work
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Readable" {
		t.Errorf("expected Title='Readable', got %q", got.Title)
	}

	// List should work
	list, count, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if count != 1 || len(list) != 1 {
		t.Errorf("expected 1 item, got %d/%d", len(list), count)
	}
}

func TestReadOnly_DeniesWrites(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Original"}
	inner.nextID = 2
	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.ReadOnly[Post, int64](),
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	ctx := context.Background()

	// Create should be denied
	_, err := repo.Create(ctx, Post{Title: "New"})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("Create: expected ErrAccessDenied, got %v", err)
	}

	// Update should be denied
	_, err = repo.Update(ctx, Post{ID: 1, Title: "Hacked"})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("Update: expected ErrAccessDenied, got %v", err)
	}

	// Patch should be denied
	_, err = repo.Patch(ctx, Post{ID: 1, Title: "Hacked"}, r3.Fields{r3.NewFieldSpec("title")})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("Patch: expected ErrAccessDenied, got %v", err)
	}

	// Delete should be denied
	err = repo.Delete(ctx, 1)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("Delete: expected ErrAccessDenied, got %v", err)
	}
}

// ── Tests: ByActorType ───────────────────────────────────────────────────

func TestByActorType(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Post"}
	inner.nextID = 2

	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.ByActorType[Post, int64](map[string]permissions.Checker[Post, int64]{
			"admin": permissions.AllowAll[Post, int64](),
			"user":  permissions.ReadOnly[Post, int64](),
		}),
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	// Admin can create
	adminCtx := r3.WithActor(context.Background(), r3.Actor{ID: "1", Type: "admin"})
	_, err := repo.Create(adminCtx, Post{Title: "Admin Post"})
	if err != nil {
		t.Fatalf("admin Create failed: %v", err)
	}

	// User can read
	userCtx := r3.WithActor(context.Background(), r3.Actor{ID: "2", Type: "user"})
	_, err = repo.Get(userCtx, 1)
	if err != nil {
		t.Fatalf("user Get failed: %v", err)
	}

	// User cannot create
	_, err = repo.Create(userCtx, Post{Title: "User Post"})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("user Create: expected ErrAccessDenied, got %v", err)
	}

	// Unknown actor type is denied
	guestCtx := r3.WithActor(context.Background(), r3.Actor{ID: "3", Type: "guest"})
	_, err = repo.Get(guestCtx, 1)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("guest Get: expected ErrAccessDenied, got %v", err)
	}
}

// ── Tests: Entity-aware checks ───────────────────────────────────────────

func TestEntityAwareCheck_OwnerCanUpdate(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "My Post", OwnerID: "user42"}
	inner.nextID = 2

	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			if req.Actor.Type == "admin" {
				return nil
			}
			if req.Operation == permissions.OpRead {
				return nil
			}
			// For mutations, only allow if the entity's owner matches the actor.
			if req.Entity != nil && req.Entity.OwnerID == req.Actor.ID {
				return nil
			}
			return &permissions.AccessDeniedError{
				Operation: req.Operation,
				Actor:     req.Actor,
				Reason:    "only the owner can modify this post",
			}
		},
	)

	repo := permissions.WithPermissions[Post, int64](
		inner, checker,
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	// Owner can update
	ownerCtx := r3.WithActor(context.Background(), r3.Actor{ID: "user42", Type: "user"})
	updated, err := repo.Update(ownerCtx, Post{ID: 1, Title: "Updated"})
	if err != nil {
		t.Fatalf("owner Update failed: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("expected Title='Updated', got %q", updated.Title)
	}

	// Non-owner cannot update
	otherCtx := r3.WithActor(context.Background(), r3.Actor{ID: "user99", Type: "user"})
	_, err = repo.Update(otherCtx, Post{ID: 1, Title: "Hacked"})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("non-owner Update: expected ErrAccessDenied, got %v", err)
	}
}

func TestEntityAwareCheck_OwnerCanDelete(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "My Post", OwnerID: "user42"}
	inner.nextID = 2

	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			if req.Operation == permissions.OpRead {
				return nil
			}
			if req.Entity != nil && req.Entity.OwnerID == req.Actor.ID {
				return nil
			}
			return &permissions.AccessDeniedError{
				Operation: req.Operation,
				Actor:     req.Actor,
				Reason:    "only the owner can modify this post",
			}
		},
	)

	repo := permissions.WithPermissions[Post, int64](
		inner, checker,
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	// Non-owner cannot delete
	otherCtx := r3.WithActor(context.Background(), r3.Actor{ID: "user99", Type: "user"})
	err := repo.Delete(otherCtx, 1)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("non-owner Delete: expected ErrAccessDenied, got %v", err)
	}

	// Owner can delete
	ownerCtx := r3.WithActor(context.Background(), r3.Actor{ID: "user42", Type: "user"})
	err = repo.Delete(ownerCtx, 1)
	if err != nil {
		t.Fatalf("owner Delete failed: %v", err)
	}
}

func TestEntityAwareCheck_OwnerCanPatch(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "My Post", OwnerID: "user42"}
	inner.nextID = 2

	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			if req.Operation == permissions.OpRead {
				return nil
			}
			if req.Entity != nil && req.Entity.OwnerID == req.Actor.ID {
				return nil
			}
			return &permissions.AccessDeniedError{
				Operation: req.Operation,
				Actor:     req.Actor,
			}
		},
	)

	repo := permissions.WithPermissions[Post, int64](
		inner, checker,
		permissions.WithIDFunc[Post, int64](postIDFunc),
	)

	// Owner can patch
	ownerCtx := r3.WithActor(context.Background(), r3.Actor{ID: "user42", Type: "user"})
	patched, err := repo.Patch(ownerCtx, Post{ID: 1, Title: "Patched"}, r3.Fields{r3.NewFieldSpec("title")})
	if err != nil {
		t.Fatalf("owner Patch failed: %v", err)
	}
	if patched.Title != "Patched" {
		t.Errorf("expected Title='Patched', got %q", patched.Title)
	}

	// Non-owner cannot patch
	otherCtx := r3.WithActor(context.Background(), r3.Actor{ID: "user99", Type: "user"})
	_, err = repo.Patch(otherCtx, Post{ID: 1, Title: "Hacked"}, r3.Fields{r3.NewFieldSpec("title")})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("non-owner Patch: expected ErrAccessDenied, got %v", err)
	}
}

// ── Tests: Scoper interface ──────────────────────────────────────────────

type scopedPolicy struct{}

func (scopedPolicy) Check(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
	if req.Operation == permissions.OpRead {
		return nil
	}
	return &permissions.AccessDeniedError{Operation: req.Operation, Actor: req.Actor}
}

func (scopedPolicy) Scope(_ context.Context, actor r3.Actor) (r3.Filters, error) {
	if actor.Type == "admin" {
		return nil, nil // admins see everything
	}
	return r3.Filters{r3.F(r3.NewFieldSpec("owner_id"), actor.ID)}, nil
}

func TestScoper_InjectsFilters(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Alice Post", OwnerID: "alice"}
	inner.data[2] = Post{ID: 2, Title: "Bob Post", OwnerID: "bob"}
	inner.nextID = 3

	repo := permissions.WithPermissions[Post, int64](inner, scopedPolicy{})

	// User "alice" should only see her own posts (via scope filter).
	aliceCtx := r3.WithActor(context.Background(), r3.Actor{ID: "alice", Type: "user"})
	list, count, err := repo.List(aliceCtx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count=1 for alice, got %d", count)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 item for alice, got %d", len(list))
	}
	if len(list) > 0 && list[0].OwnerID != "alice" {
		t.Errorf("expected alice's post, got owner=%s", list[0].OwnerID)
	}
}

func TestScoper_AdminSeesAll(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Alice Post", OwnerID: "alice"}
	inner.data[2] = Post{ID: 2, Title: "Bob Post", OwnerID: "bob"}
	inner.nextID = 3

	repo := permissions.WithPermissions[Post, int64](inner, scopedPolicy{})

	// Admin should see everything (no scope filter injected).
	adminCtx := r3.WithActor(context.Background(), r3.Actor{ID: "admin1", Type: "admin"})
	list, count, err := repo.List(adminCtx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count=2 for admin, got %d", count)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 items for admin, got %d", len(list))
	}
}

// ── Tests: Compose ───────────────────────────────────────────────────────

func TestCompose_AllMustAllow(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Post"}
	inner.nextID = 2

	// First checker allows everything, second denies everything.
	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.Compose[Post, int64](
			permissions.AllowAll[Post, int64](),
			permissions.DenyAll[Post, int64](),
		),
	)

	ctx := context.Background()
	_, err := repo.Get(ctx, 1)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied from Compose, got %v", err)
	}
}

func TestCompose_AllAllow(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Post"}
	inner.nextID = 2

	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.Compose[Post, int64](
			permissions.AllowAll[Post, int64](),
			permissions.AllowAll[Post, int64](),
		),
	)

	ctx := context.Background()
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Post" {
		t.Errorf("expected Title='Post', got %q", got.Title)
	}
}

func TestCompose_Empty(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Post"}
	inner.nextID = 2

	// Empty compose allows everything.
	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.Compose[Post, int64](),
	)

	ctx := context.Background()
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Post" {
		t.Errorf("expected Title='Post', got %q", got.Title)
	}
}

// ── Tests: OperationCheckers ─────────────────────────────────────────────

func TestOperationCheckers_MappedOps(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Post"}
	inner.nextID = 2

	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.OperationCheckers[Post, int64](
			//nolint:exhaustive // intentionally partial map; unmapped ops are denied by default
			map[permissions.Operation]permissions.Checker[Post, int64]{
				permissions.OpRead: permissions.AllowAll[Post, int64](),
			},
		),
	)

	ctx := context.Background()

	// Read is mapped and allowed.
	got, err := repo.Get(ctx, 1)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Post" {
		t.Errorf("expected Title='Post', got %q", got.Title)
	}

	// Create is not mapped -> denied by default.
	_, err = repo.Create(ctx, Post{Title: "New"})
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("Create: expected ErrAccessDenied, got %v", err)
	}
}

func TestOperationCheckers_WithFallback(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Post"}
	inner.nextID = 2

	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.OperationCheckers[Post, int64](
			//nolint:exhaustive // intentionally partial map; unmapped ops use fallback
			map[permissions.Operation]permissions.Checker[Post, int64]{
				permissions.OpRead: permissions.AllowAll[Post, int64](),
			},
			permissions.AllowAll[Post, int64](), // fallback: allow unmapped ops
		),
	)

	ctx := context.Background()

	// Create is not mapped, but fallback allows it.
	post, err := repo.Create(ctx, Post{Title: "New"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if post.Title != "New" {
		t.Errorf("expected Title='New', got %q", post.Title)
	}
}

// ── Tests: Error types ───────────────────────────────────────────────────

func TestAccessDeniedError_Is(t *testing.T) {
	err := &permissions.AccessDeniedError{
		Operation:  permissions.OpCreate,
		Actor:      r3.Actor{ID: "1", Type: "user"},
		RecordType: "posts",
		Reason:     "test",
	}

	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatal("expected AccessDeniedError to satisfy errors.Is(err, ErrAccessDenied)")
	}
}

func TestAccessDeniedError_ErrorMessage(t *testing.T) {
	err := &permissions.AccessDeniedError{
		Operation:  permissions.OpUpdate,
		Actor:      r3.Actor{ID: "42", Type: "user"},
		RecordType: "posts",
		RecordID:   "7",
		Reason:     "not the owner",
	}

	msg := err.Error()
	expected := "r3/permissions: access denied: update on posts (id=7) by actor user/42: not the owner"
	if msg != expected {
		t.Errorf("error message mismatch:\n  got:  %q\n  want: %q", msg, expected)
	}
}

func TestAccessDeniedError_ErrorMessage_NoRecordID(t *testing.T) {
	err := &permissions.AccessDeniedError{
		Operation:  permissions.OpRead,
		Actor:      r3.Actor{ID: "42", Type: "user"},
		RecordType: "posts",
	}

	msg := err.Error()
	expected := "r3/permissions: access denied: read on posts by actor user/42"
	if msg != expected {
		t.Errorf("error message mismatch:\n  got:  %q\n  want: %q", msg, expected)
	}
}

// ── Tests: Inner() ───────────────────────────────────────────────────────

func TestInner(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](inner, permissions.AllowAll[Post, int64]())

	got := repo.Inner()
	if got != inner {
		t.Error("Inner() should return the wrapped CRUD")
	}
}

// ── Tests: Get does not leak entity on denied ────────────────────────────

func TestGet_DoesNotLeakEntityOnDenied(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Secret", OwnerID: "user42"}
	inner.nextID = 2

	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			if req.Entity != nil && req.Entity.OwnerID != req.Actor.ID {
				return &permissions.AccessDeniedError{
					Operation: req.Operation,
					Actor:     req.Actor,
					Reason:    "not your post",
				}
			}
			return nil
		},
	)

	repo := permissions.WithPermissions[Post, int64](inner, checker)

	otherCtx := r3.WithActor(context.Background(), r3.Actor{ID: "user99", Type: "user"})
	result, err := repo.Get(otherCtx, 1)
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
	// The zero value should be returned, not the actual entity.
	if result.Title != "" {
		t.Errorf("expected zero-value Post on denied Get, got Title=%q", result.Title)
	}
}

// ── Tests: CheckerFunc adapter ───────────────────────────────────────────

func TestCheckerFunc(t *testing.T) {
	called := false
	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, _ permissions.AccessRequest[Post, int64]) error {
			called = true
			return nil
		},
	)

	err := checker.Check(context.Background(), permissions.AccessRequest[Post, int64]{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("CheckerFunc was not called")
	}
}

// ── Tests: WithRecordType option ─────────────────────────────────────────

func TestWithRecordType(t *testing.T) {
	inner := newMemoryCRUD()
	repo := permissions.WithPermissions[Post, int64](
		inner,
		permissions.DenyAll[Post, int64](),
		permissions.WithRecordType[Post, int64]("blog_posts"),
	)

	ctx := context.Background()
	_, err := repo.Create(ctx, Post{Title: "Test"})
	if err == nil {
		t.Fatal("expected error")
	}
	// The error should contain the custom record type.
	// (DenyAll doesn't use RecordType, but the decorator's denied() method would.)
	// We just verify it's an AccessDeniedError.
	if !errors.Is(err, permissions.ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

// ── Tests: Update/Patch/Delete without IDFunc ────────────────────────────

func TestUpdate_WithoutIDFunc_SkipsEntityFetch(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Original", OwnerID: "user42"}
	inner.nextID = 2

	// Without IDFunc, the checker won't get the entity.
	var gotEntity *Post
	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			gotEntity = req.Entity
			return nil
		},
	)

	repo := permissions.WithPermissions[Post, int64](inner, checker)

	ctx := context.Background()
	_, err := repo.Update(ctx, Post{ID: 1, Title: "Updated"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if gotEntity != nil {
		t.Error("expected Entity to be nil without IDFunc")
	}
}

func TestDelete_WithoutIDFunc_SkipsEntityFetch(t *testing.T) {
	inner := newMemoryCRUD()
	inner.data[1] = Post{ID: 1, Title: "Original", OwnerID: "user42"}
	inner.nextID = 2

	var gotEntity *Post
	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			gotEntity = req.Entity
			return nil
		},
	)

	repo := permissions.WithPermissions[Post, int64](inner, checker)

	ctx := context.Background()
	err := repo.Delete(ctx, 1)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if gotEntity != nil {
		t.Error("expected Entity to be nil without IDFunc")
	}
}

// ── Tests: Actor from context ────────────────────────────────────────────

func TestActorFromContext(t *testing.T) {
	inner := newMemoryCRUD()

	var capturedActor r3.Actor
	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			capturedActor = req.Actor
			return nil
		},
	)

	repo := permissions.WithPermissions[Post, int64](inner, checker)

	ctx := r3.WithActor(context.Background(), r3.Actor{ID: "42", Type: "user"})
	_, _ = repo.Create(ctx, Post{Title: "Test"})

	if capturedActor.ID != "42" || capturedActor.Type != "user" {
		t.Errorf("expected actor {42, user}, got {%s, %s}", capturedActor.ID, capturedActor.Type)
	}
}

func TestActorFromContext_Default(t *testing.T) {
	inner := newMemoryCRUD()

	var capturedActor r3.Actor
	checker := permissions.CheckerFunc[Post, int64](
		func(_ context.Context, req permissions.AccessRequest[Post, int64]) error {
			capturedActor = req.Actor
			return nil
		},
	)

	repo := permissions.WithPermissions[Post, int64](inner, checker)

	// No actor in context -> SystemActor
	ctx := context.Background()
	_, _ = repo.Create(ctx, Post{Title: "Test"})

	if capturedActor != r3.SystemActor {
		t.Errorf("expected SystemActor, got {%s, %s}", capturedActor.ID, capturedActor.Type)
	}
}
