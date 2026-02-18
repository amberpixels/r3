package r3

// Option is a functional option for configuring R3 repositories.
// Pass options to engine/driver constructors to customize behavior.
type Option func(*Options)

// Options holds the resolved configuration for a repository.
// Engine and driver constructors call [ResolveOptions] to apply
// functional options and get the final values.
type Options struct {
	// Config is the framework-level configuration.
	Config Config
}

// DefaultOptions returns Options initialized with sensible defaults.
func DefaultOptions() Options {
	return Options{
		Config: DefaultConfig(),
	}
}

// ResolveOptions applies functional options to the default [Options]
// and returns the result.
func ResolveOptions(opts ...Option) Options {
	o := DefaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WithConfig sets the R3 framework-level configuration.
func WithConfig(cfg Config) Option {
	return func(o *Options) {
		o.Config = cfg
	}
}
