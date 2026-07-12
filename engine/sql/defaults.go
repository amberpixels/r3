package enginesql

import (
	"github.com/amberpixels/r3"
)

// Defaults is an alias for r3.Defaults.
//
// Deprecated: Use r3.Defaults directly.
type Defaults = r3.Defaults

// NewDefaults returns Defaults with reasonable default queries.
//
// Deprecated: Use r3.NewDefaults directly.
func NewDefaults() Defaults { return r3.NewDefaults() }

// DefaultsManager is an alias for r3.DefaultsManager.
//
// Deprecated: Use r3.DefaultsManager directly.
type DefaultsManager = r3.DefaultsManager

// NewDefaultsManager creates a DefaultsManager with reasonable defaults.
//
// Deprecated: Use r3.NewDefaultsManager directly.
func NewDefaultsManager() DefaultsManager { return r3.NewDefaultsManager() }
