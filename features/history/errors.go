package history

import "errors"

// ErrIDFuncRequired is returned when WithHistory is called without an IDFunc,
// which the decorator needs to extract an entity's primary key.
var ErrIDFuncRequired = errors.New("r3history: IDFunc is required; use WithIDFunc option")
