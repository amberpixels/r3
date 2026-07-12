package r3gorm

import (
	"errors"

	"github.com/amberpixels/r3"
	"gorm.io/gorm"
)

// ErrRawNotSupported is returned by [RawOf] and [DBOf] when the repository is
// not (and does not unwrap to) a gorm-backed repository. It is driver-specific
// and deliberately lives here rather than in core r3.
var ErrRawNotSupported = errors.New("r3gorm: raw driver access not supported")

// RawAccessor reaches the typed gorm builder ([GormRaw], via Raw) or the *gorm.DB
// (via DB) of a gorm-backed repo. *GormCRUD[T, ID] already satisfies it; the
// interface exists so [RawOf]/[DBOf] can find it through a decorator chain.
type RawAccessor[T any, ID comparable] interface {
	Raw() *GormRaw[T, ID]
	DB() *gorm.DB
}

var _ RawAccessor[any, int64] = (*GormCRUD[any, int64])(nil)

// RawOf returns the typed gorm raw-builder for repo, unwrapping any decorator
// chain (via [r3.As]) down to the gorm backend, or [ErrRawNotSupported] if repo is
// not gorm-backed. It saves a hand-written type assertion when a service holds its
// repo as the backend-agnostic r3.CRUD[T, ID].
//
// The builder runs BELOW the feature decorators (history, permissions, i18n, ...):
// reads are fine, but a WRITE through it is neither audited, permission-checked,
// nor stale-marked. Prefer a high-level method for writes; use RawOf only for reads
// or queries no repo method can express.
func RawOf[T any, ID comparable](repo r3.CRUD[T, ID]) (*GormRaw[T, ID], error) {
	if ra, ok := r3.As[RawAccessor[T, ID]](repo); ok {
		return ra.Raw(), nil
	}
	return nil, ErrRawNotSupported
}

// DBOf returns the underlying *gorm.DB (full raw access: .Exec, .Raw(sql), ...),
// with the same unwrap and error contract as [RawOf]. Use it only when [GormRaw]'s
// builder callback is not enough.
//
// The [RawOf] caveat applies more sharply: the *gorm.DB bypasses the entire feature
// stack. Prefer high-level methods for writes; use DBOf for reads or queries that
// neither a repo method nor RawOf's builder can express.
func DBOf[T any, ID comparable](repo r3.CRUD[T, ID]) (*gorm.DB, error) {
	if ra, ok := r3.As[RawAccessor[T, ID]](repo); ok {
		return ra.DB(), nil
	}
	return nil, ErrRawNotSupported
}
