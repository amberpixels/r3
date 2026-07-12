package enginefile

import "github.com/amberpixels/r3"

// Option configures a BaseCRUD instance.
type Option func(*config)

// config holds all configuration values before constructing BaseCRUD.
type config struct {
	baseDir       string
	codec         Codec
	directoryMode bool
	filePath      string // explicit path override
	r3Opts        []r3.Option
}

// WithBaseDir sets the base directory; each entity type gets a file (or
// subdirectory) under it.
func WithBaseDir(dir string) Option {
	return func(c *config) {
		c.baseDir = dir
	}
}

// WithCodec sets the serialization codec ([JSONCodec], [YAMLCodec], or [NewCodec]).
func WithCodec(codec Codec) Option {
	return func(c *config) {
		c.codec = codec
	}
}

// WithDirectoryMode stores each entity in its own file under a resource-named
// directory (cities/1.json, cities/2.json) instead of a single cities.json.
func WithDirectoryMode() Option {
	return func(c *config) {
		c.directoryMode = true
	}
}

// WithFilePath overrides the auto-derived path (resource name + codec
// extension). Single-file mode only.
func WithFilePath(path string) Option {
	return func(c *config) {
		c.filePath = path
	}
}

// WithR3Options passes framework-level r3.Option values (e.g. [r3.WithConfig])
// to the file-based CRUD constructor.
func WithR3Options(opts ...r3.Option) Option {
	return func(c *config) {
		c.r3Opts = append(c.r3Opts, opts...)
	}
}
