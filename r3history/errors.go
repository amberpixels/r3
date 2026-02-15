package r3history

import "errors"

// ErrIDFuncRequired is returned when WithHistory is called without an IDFunc.
// The decorator needs to know how to extract the primary key from an entity.
var ErrIDFuncRequired = errors.New("r3history: IDFunc is required; use WithIDFunc option")
