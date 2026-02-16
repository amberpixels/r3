package softdelete

import "errors"

// ErrNotSoftDeletable is returned when Restore or HardDelete is called on a
// decorator whose inner CRUD does not implement the SoftDeleter interface.
var ErrNotSoftDeletable = errors.New("r3/softdelete: inner CRUD does not implement SoftDeleter")
