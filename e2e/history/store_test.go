package history_test

import (
	"context"
	"sync"

	"github.com/amberpixels/r3"
	"github.com/amberpixels/r3/features/history"
)

// changeRecordStore is the minimum r3.CRUD[history.ChangeRecord, string] the
// phantom-diff test needs: history only ever appends through it, and the test
// reads everything back in insertion order. features/history's own suite has a
// fuller in-memory store, but that one lives with the tests that need its filter
// and sort support, and duplicating forty lines of slice scanning beats exporting
// two hundred.
type changeRecordStore struct {
	mu      sync.Mutex
	records []history.ChangeRecord
}

func newChangeRecordStore() *changeRecordStore { return &changeRecordStore{} }

func (s *changeRecordStore) Create(
	_ context.Context, rec history.ChangeRecord,
) (history.ChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
	return rec, nil
}

func (s *changeRecordStore) Get(
	_ context.Context, id string, _ ...r3.Query,
) (history.ChangeRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, rec := range s.records {
		if rec.ID == id {
			return rec, nil
		}
	}
	return history.ChangeRecord{}, history.ErrRecordNotFound
}

func (s *changeRecordStore) List(
	_ context.Context, _ ...r3.Query,
) ([]history.ChangeRecord, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]history.ChangeRecord, len(s.records))
	copy(out, s.records)
	return out, int64(len(out)), nil
}

func (s *changeRecordStore) Count(ctx context.Context, q ...r3.Query) (int64, error) {
	_, n, err := s.List(ctx, q...)
	return n, err
}

func (s *changeRecordStore) Update(
	_ context.Context, rec history.ChangeRecord,
) (history.ChangeRecord, error) {
	return rec, nil
}

func (s *changeRecordStore) Patch(
	_ context.Context, rec history.ChangeRecord, _ r3.Fields,
) (history.ChangeRecord, error) {
	return rec, nil
}

func (s *changeRecordStore) Delete(_ context.Context, _ string) error { return nil }

// forRecord returns the records filed against one entity, in the order history
// wrote them, which is the order their versions run in.
func (s *changeRecordStore) forRecord(recordType, id string) []history.ChangeRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []history.ChangeRecord
	for _, rec := range s.records {
		if rec.RecordType == recordType && rec.RecordID == id {
			out = append(out, rec)
		}
	}
	return out
}
