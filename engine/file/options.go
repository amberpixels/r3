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

// WithBaseDir sets the base directory for file storage.
// Each entity type will be stored in a file (or subdirectory) under this path.
func WithBaseDir(dir string) Option {
	return func(c *config) {
		c.baseDir = dir
	}
}

// WithCodec sets the codec used for serialization.
// Use JSONCodec(), YAMLCodec(), or NewCodec() for a custom codec.
func WithCodec(codec Codec) Option {
	return func(c *config) {
		c.codec = codec
	}
}

// WithDirectoryMode enables directory mode where each entity is stored
// in a separate file within a directory named after the resource.
// For example: cities/1.json, cities/2.json instead of cities.json.
func WithDirectoryMode() Option {
	return func(c *config) {
		c.directoryMode = true
	}
}

// WithFilePath sets an explicit file path, overriding the auto-derived
// path from resource name + codec extension. Only valid in single-file mode.
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
