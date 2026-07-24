package r3sqlite3_test

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/history"
)

// memChangeStore is a minimal in-memory r3.CRUD[history.ChangeRecord, string]
// used to assert that audited writes still produce a change record. Only the
// methods the history decorator exercises (Create, List) carry real behavior.
type memChangeStore struct {
	mu      sync.Mutex
	records []history.ChangeRecord
}

func (m *memChangeStore) Create(_ context.Context, rec history.ChangeRecord) (history.ChangeRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, rec)
	return rec, nil
}

func (m *memChangeStore) List(_ context.Context, _ ...r3.Query) ([]history.ChangeRecord, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]history.ChangeRecord(nil), m.records...)
	// Latest version first, matching the history store's version query.
	sort.Slice(out, func(i, j int) bool { return out[i].Version > out[j].Version })
	return out, int64(len(out)), nil
}

func (m *memChangeStore) Get(_ context.Context, _ string, _ ...r3.Query) (history.ChangeRecord, error) {
	return history.ChangeRecord{}, r3.ErrNotFound
}

func (m *memChangeStore) Count(_ context.Context, _ ...r3.Query) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.records)), nil
}

func (m *memChangeStore) Update(_ context.Context, rec history.ChangeRecord) (history.ChangeRecord, error) {
	return rec, nil
}

func (m *memChangeStore) Patch(_ context.Context, rec history.ChangeRecord, _ r3.Fields) (history.ChangeRecord, error) {
	return rec, nil
}
func (m *memChangeStore) Delete(_ context.Context, _ string) error { return nil }

// TestSchema_SystemWriter_StaysAudited proves the §2.5 contract: a SystemWriter
// bypass write of a user-immutable column still flows through the history
// decorator (it is audited) — only the engine's capability check is skipped.
func TestSchema_SystemWriter_StaysAudited(t *testing.T) {
	base := setupWidgets(t)
	store := &memChangeStore{}

	repo := history.WithHistory[widget, int64](base, store,
		history.WithIDFunc[widget, int64](func(w widget) int64 { return w.ID }),
	)

	ctx := context.Background()
	w, err := repo.Create(ctx, widget{Title: "a", Slug: "a"})
	require.NoError(t, err)

	// Worker syncs the feed column through the audited chain.
	w.Population = 4242
	_, err = r3.SystemWriter[widget, int64](repo).Update(ctx, w)
	require.NoError(t, err)

	// The engine wrote the readonly column (bypass took effect)...
	got, err := base.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, 4242, got.Population)

	// ...and the history decorator still recorded an update (write stayed audited).
	store.mu.Lock()
	defer store.mu.Unlock()
	var sawUpdate bool
	for _, rec := range store.records {
		if rec.Action == history.ActionUpdate {
			sawUpdate = true
		}
	}
	require.True(t, sawUpdate, "SystemWriter update must still be recorded in history")
}
