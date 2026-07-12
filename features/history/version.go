package history

import (
	"context"
	"hash/fnv"
	"sync"

	"github.com/amberpixels/r3"
)

// versionLockStripes bounds the mutex count; records stripe across them by key,
// so different records write concurrently while the same record serializes.
const versionLockStripes = 64

// versionLocker serializes the read-modify-write of per-record version numbers.
//
// nextVersion (read latest, add one) followed by store.Create is not atomic, so
// two concurrent writers to the same record could read the same latest version
// and be assigned duplicate numbers, corrupting the monotonic sequence that
// Reconstruct/RevertTo rely on. A per-record lock across nextVersion+Create
// closes that window.
//
// This only guards within one process. Multiple processes (or decorator
// instances) writing the same store are not coordinated; that needs a unique
// constraint on (record_type, record_id, version), which not every backend has.
type versionLocker struct {
	stripes [versionLockStripes]sync.Mutex
}

func newVersionLocker() *versionLocker { return &versionLocker{} }

// acquire locks the stripe for the given record key and returns its unlock func.
func (v *versionLocker) acquire(recordType, recordID string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(recordType))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(recordID))
	m := &v.stripes[h.Sum32()%versionLockStripes]
	m.Lock()
	return m.Unlock
}

// persistVersioned assigns a collision-resistant ID and the next monotonic
// version, then persists via store.Create under a per-record lock so concurrent
// writers cannot get duplicate versions. Returns the record with ID and Version set.
func persistVersioned(
	ctx context.Context,
	store r3.CRUD[ChangeRecord, string],
	locker *versionLocker,
	record ChangeRecord,
) (ChangeRecord, error) {
	release := locker.acquire(record.RecordType, record.RecordID)
	defer release()

	record.Version = nextVersion(ctx, store, record.RecordType, record.RecordID)
	record.ID = generateID()

	_, err := store.Create(ctx, record)
	return record, err
}
