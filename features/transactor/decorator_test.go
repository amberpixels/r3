package transactor_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/transactor"
	"github.com/expectto/be"
)

// ── Test entity ──────────────────────────────────────────────────────────

type User struct {
	ID    int64  `db:"id,pk" json:"id"`
	Name  string `db:"name"  json:"name"`
	Email string `db:"email" json:"email"`
}

// ── In-memory CRUD mock (no transaction support) ─────────────────────────

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

func (m *plainCRUD) Count(ctx context.Context, q ...r3.Query) (int64, error) {
	_, n, err := m.List(ctx, q...)
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
	if _, ok := m.data[entity.ID]; !ok {
		return User{}, fmt.Errorf("not found: %d", entity.ID)
	}
	m.data[entity.ID] = entity
	return entity, nil
}

func (m *plainCRUD) Patch(_ context.Context, entity User, fields r3.Fields) (User, error) {
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

func (m *plainCRUD) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[id]; !ok {
		return fmt.Errorf("not found: %d", id)
	}
	delete(m.data, id)
	return nil
}

// ── In-memory CRUD mock WITH transaction support ─────────────────────────

type txCRUD struct {
	plainCRUD
}

func newTxCRUD() *txCRUD {
	return &txCRUD{plainCRUD: plainCRUD{data: make(map[int64]User), nextID: 1}}
}

// memTxCRUD is the transactional CRUD returned by BeginTx.
// It wraps the parent txCRUD and records whether Commit/Rollback was called.
type memTxCRUD struct {
	parent     *txCRUD
	committed  bool
	rolledBack bool
}

var _ r3.TxCRUD[User, int64] = &memTxCRUD{}

func (t *memTxCRUD) Create(ctx context.Context, entity User) (User, error) {
	return t.parent.Create(ctx, entity)
}

func (t *memTxCRUD) Get(ctx context.Context, id int64, q ...r3.Query) (User, error) {
	return t.parent.Get(ctx, id, q...)
}

func (t *memTxCRUD) Count(ctx context.Context, q ...r3.Query) (int64, error) {
	_, n, err := t.List(ctx, q...)
	return n, err
}

func (t *memTxCRUD) List(ctx context.Context, q ...r3.Query) ([]User, int64, error) {
	return t.parent.List(ctx, q...)
}

func (t *memTxCRUD) Update(ctx context.Context, entity User) (User, error) {
	return t.parent.Update(ctx, entity)
}

func (t *memTxCRUD) Patch(ctx context.Context, entity User, fields r3.Fields) (User, error) {
	return t.parent.Patch(ctx, entity, fields)
}

func (t *memTxCRUD) Delete(ctx context.Context, id int64) error {
	return t.parent.Delete(ctx, id)
}

func (t *memTxCRUD) Commit() error {
	t.committed = true
	return nil
}

func (t *memTxCRUD) Rollback() error {
	t.rolledBack = true
	return nil
}

// BeginTx implements r3.Transactor for the txCRUD mock.
func (m *txCRUD) BeginTx(_ context.Context) (r3.TxCRUD[User, int64], error) {
	return &memTxCRUD{parent: m}, nil
}

// Compile-time check.
var _ r3.Transactor[User, int64] = &txCRUD{}

// ── Tests: CRUD passthrough ──────────────────────────────────────────────

func TestCRUD_PassthroughCreate(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	user, err := repo.Create(ctx, User{Name: "Alice", Email: "alice@example.com"})
	be.NoError(t, err)
	be.RequireThat(t, user.ID, be.NonZero())
	be.AssertThat(t, user.Name, be.Eq("Alice"))
}

func TestCRUD_PassthroughGet(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	created, _ := repo.Create(ctx, User{Name: "Bob", Email: "bob@example.com"})

	got, err := repo.Get(ctx, created.ID)
	be.NoError(t, err)
	be.AssertThat(t, got.Name, be.Eq("Bob"))
}

func TestCRUD_PassthroughList(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	_, _ = repo.Create(ctx, User{Name: "A"})
	_, _ = repo.Create(ctx, User{Name: "B"})

	list, count, err := repo.List(ctx)
	be.NoError(t, err)
	be.AssertThat(t, count, be.Eq(int64(2)))
	be.AssertThat(t, list, be.HaveLength(2))
}

func TestCRUD_PassthroughUpdate(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Carol"})
	user.Name = "Carol Updated"
	updated, err := repo.Update(ctx, user)
	be.NoError(t, err)
	be.AssertThat(t, updated.Name, be.Eq("Carol Updated"))
}

func TestCRUD_PassthroughPatch(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Dave", Email: "dave@example.com"})
	user.Email = "newemail@example.com"
	patched, err := repo.Patch(ctx, user, r3.Fields{r3.NewFieldSpec("email")})
	be.NoError(t, err)
	be.AssertThat(t, patched.Email, be.Eq("newemail@example.com"))
}

func TestCRUD_PassthroughDelete(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	user, _ := repo.Create(ctx, User{Name: "Eve"})

	err := repo.Delete(ctx, user.ID)
	be.NoError(t, err)

	_, err = repo.Get(ctx, user.ID)
	be.Error(t, err)
}

// ── Tests: Transaction support ───────────────────────────────────────────

func TestSupportsTransactions_True(t *testing.T) {
	inner := newTxCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	be.RequireThat(t, repo.SupportsTransactions(), be.True())
}

func TestSupportsTransactions_False(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	be.RequireThat(t, repo.SupportsTransactions(), be.False())
}

func TestBeginTx_Supported(t *testing.T) {
	inner := newTxCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	tx, err := repo.BeginTx(ctx)
	be.NoError(t, err)

	// Should be able to use the tx CRUD
	user, err := tx.Create(ctx, User{Name: "TxUser"})
	be.NoError(t, err)
	be.AssertThat(t, user.Name, be.Eq("TxUser"))

	err = tx.Commit()
	be.NoError(t, err)
}

func TestBeginTx_NotSupported(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	_, err := repo.BeginTx(ctx)
	be.ErrorIs(t, err, r3.ErrTransactionsNotSupported)
}

func TestInTx_Supported(t *testing.T) {
	inner := newTxCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	var createdID int64

	err := repo.InTx(ctx, func(tx r3.CRUD[User, int64]) error {
		user, err := tx.Create(ctx, User{Name: "InTxUser"})
		if err != nil {
			return err
		}
		createdID = user.ID
		return nil
	})
	be.NoError(t, err)
	be.RequireThat(t, createdID, be.NonZero())
}

func TestInTx_NotSupported(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	err := repo.InTx(ctx, func(_ r3.CRUD[User, int64]) error {
		return nil
	})
	be.ErrorIs(t, err, r3.ErrTransactionsNotSupported)
}

func TestInTx_RollbackOnError(t *testing.T) {
	inner := newTxCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	ctx := context.Background()
	fnErr := errors.New("something went wrong")

	err := repo.InTx(ctx, func(_ r3.CRUD[User, int64]) error {
		return fnErr
	})
	be.ErrorIs(t, err, fnErr)
}

func TestInner(t *testing.T) {
	inner := newPlainCRUD()
	repo := transactor.WithTransactor[User, int64](inner)

	got := repo.Inner()
	be.AssertThat(t, got, be.Eq(inner))
}

// countingCRUD is a minimal decorator that counts Create calls and participates
// in the Unwrap/Rewrap chain protocol. It is used to prove that decorators are
// re-applied on top of the transaction and run for writes inside InTx/BeginTx.
type countingCRUD struct {
	inner   r3.CRUD[User, int64]
	creates *int32
}

func (c *countingCRUD) Create(ctx context.Context, e User) (User, error) {
	atomic.AddInt32(c.creates, 1)
	return c.inner.Create(ctx, e)
}

func (c *countingCRUD) Get(ctx context.Context, id int64, q ...r3.Query) (User, error) {
	return c.inner.Get(ctx, id, q...)
}

func (c *countingCRUD) List(ctx context.Context, q ...r3.Query) ([]User, int64, error) {
	return c.inner.List(ctx, q...)
}

func (c *countingCRUD) Count(ctx context.Context, q ...r3.Query) (int64, error) {
	return c.inner.Count(ctx, q...)
}

func (c *countingCRUD) Update(ctx context.Context, e User) (User, error) {
	return c.inner.Update(ctx, e)
}

func (c *countingCRUD) Patch(ctx context.Context, e User, f r3.Fields) (User, error) {
	return c.inner.Patch(ctx, e, f)
}

func (c *countingCRUD) Delete(ctx context.Context, id int64) error {
	return c.inner.Delete(ctx, id)
}
func (c *countingCRUD) Unwrap() r3.CRUD[User, int64] { return c.inner }
func (c *countingCRUD) Rewrap(inner r3.CRUD[User, int64]) r3.CRUD[User, int64] {
	return &countingCRUD{inner: inner, creates: c.creates} // share the counter
}

// TestInTx_DecoratorsRunInsideTransaction verifies that decorators between the
// transactor and the backend are re-applied on top of the transaction, so their
// behaviour runs for writes inside InTx (C2). Before the fix, InTx handed the
// raw backend tx to fn, bypassing every decorator.
func TestInTx_DecoratorsRunInsideTransaction(t *testing.T) {
	backend := newTxCRUD()
	var creates int32
	decorated := &countingCRUD{inner: backend, creates: &creates}
	repo := transactor.WithTransactor[User, int64](decorated)

	ctx := context.Background()
	err := repo.InTx(ctx, func(tx r3.CRUD[User, int64]) error {
		_, cErr := tx.Create(ctx, User{Name: "TxUser"})
		return cErr
	})
	be.NoError(t, err)
	be.RequireThat(t, atomic.LoadInt32(&creates), be.Eq(int32(1)))
}

// TestBeginTx_DecoratorsRunInsideTransaction verifies the same for the BeginTx
// entry point.
func TestBeginTx_DecoratorsRunInsideTransaction(t *testing.T) {
	backend := newTxCRUD()
	var creates int32
	decorated := &countingCRUD{inner: backend, creates: &creates}
	repo := transactor.WithTransactor[User, int64](decorated)

	ctx := context.Background()
	tx, err := repo.BeginTx(ctx)
	be.NoError(t, err)
	_, err = tx.Create(ctx, User{Name: "TxUser"})
	be.NoError(t, err)
	err = tx.Commit()
	be.NoError(t, err)
	be.RequireThat(t, atomic.LoadInt32(&creates), be.Eq(int32(1)))
}
