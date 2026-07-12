// Package enginefile is the filesystem CRUD engine: it reads and writes entities
// to JSON or YAML files and evaluates every query feature (filters, sorts,
// pagination, field selection, soft-delete) in memory. Serialization goes through
// a pluggable [Codec]; two layout modes are supported:
//
//   - Single-file mode (default): one file per collection (e.g. cities.json)
//   - Directory mode: one file per entity (e.g. cities/1.json, cities/2.json)
//
// Usage:
//
//	repo, err := enginefile.New[City, int](
//	    enginefile.WithBaseDir("./data"),
//	    enginefile.WithCodec(enginefile.JSONCodec()),
//	    enginefile.WithIDGenerator(enginefile.IncrementIDGen[int]()),
//	)
//
// Codecs can wrap an alternative JSON library via [NewCodec].
package enginefile
