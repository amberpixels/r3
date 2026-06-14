package history

import (
	"context"
	"hash/fnv"
	"sync"

	"github.com/amberpixels/r3"
)

// versionLockStripes bounds the number of mutexes used to serialize per-record
// version assignment. Records are striped across the mutexes by key, so writes
// to different records run concurrently while writes to the same record are
// serialized.
const versionLockStripes = 64

// versionLocker serializes the read-modify-write of per-record version numbers.
//
// nextVersion (a "read the latest version, add one" query) followed by
// store.Create is not atomic on its own, so two concurrent writers to the same
// record could read the same latest version and be assigned duplicate version
// numbers — corrupting the monotonic sequence that Reconstruct/RevertTo rely on.
// Holding a per-record lock across nextVersion+Create closes that window.
//
// This protects concurrency within a single process. Multiple processes (or
// decorator instances) writing the same store are not coordinated; that would
// require a unique constraint on (record_type, record_id, version) in the store,
// which not every backend supports.
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
// version to record, then persists it via store.Create. The version assignment
// and the create run under a per-record lock so concurrent writers cannot be
// assigned duplicate versions. The persisted record (with ID and Version set)
// is returned.
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
