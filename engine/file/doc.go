// Package enginefile provides a file-based CRUD engine for r3.
//
// It implements the r3.CRUD interface by reading and writing entities
// to JSON or YAML files on disk. The engine supports pluggable codecs
// for serialization, in-memory filtering/sorting/pagination, and two
// file layout modes:
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
// The engine supports all r3 query features: filters, sorts, pagination,
// field selection, soft-delete, and default query merging.
//
// Codecs can be customized to use alternative JSON libraries:
//
//	import goccy "github.com/goccy/go-json"
//
//	codec := enginefile.NewCodec(".json",
//	    func(w io.Writer) enginefile.Encoder { return goccy.NewEncoder(w) },
//	    func(r io.Reader) enginefile.Decoder { return goccy.NewDecoder(r) },
//	)
package enginefile
