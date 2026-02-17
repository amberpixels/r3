package enginefile

import (
	"encoding/json"
	"io"

	"gopkg.in/yaml.v3"
)

// Encoder writes encoded values to an output stream.
type Encoder interface {
	Encode(v any) error
}

// Decoder reads and decodes values from an input stream.
type Decoder interface {
	Decode(v any) error
}

// Codec handles serialization of entity slices to/from files.
// Implementations work with io.Reader/io.Writer for streaming I/O.
type Codec interface {
	// NewEncoder creates a new encoder writing to w.
	NewEncoder(w io.Writer) Encoder
	// NewDecoder creates a new decoder reading from r.
	NewDecoder(r io.Reader) Decoder
	// FileExtension returns the file extension including the dot (e.g. ".json", ".yaml").
	FileExtension() string
}

// jsonCodec implements Codec using encoding/json.
type jsonCodec struct {
	indent string
}

func (c *jsonCodec) NewEncoder(w io.Writer) Encoder {
	enc := json.NewEncoder(w)
	if c.indent != "" {
		enc.SetIndent("", c.indent)
	}
	return enc
}

func (c *jsonCodec) NewDecoder(r io.Reader) Decoder {
	return json.NewDecoder(r)
}

func (c *jsonCodec) FileExtension() string {
	return ".json"
}

// JSONCodec returns a Codec that uses encoding/json with pretty-printed output.
func JSONCodec() Codec {
	return &jsonCodec{indent: "  "}
}

// JSONCodecCompact returns a Codec that uses encoding/json without indentation.
func JSONCodecCompact() Codec {
	return &jsonCodec{}
}

// yamlCodec implements Codec using gopkg.in/yaml.v3.
type yamlCodec struct{}

func (c *yamlCodec) NewEncoder(w io.Writer) Encoder {
	return yaml.NewEncoder(w)
}

func (c *yamlCodec) NewDecoder(r io.Reader) Decoder {
	return yaml.NewDecoder(r)
}

func (c *yamlCodec) FileExtension() string {
	return ".yaml"
}

// YAMLCodec returns a Codec that uses gopkg.in/yaml.v3.
func YAMLCodec() Codec {
	return &yamlCodec{}
}

// customCodec implements Codec using user-provided factory functions.
type customCodec struct {
	ext        string
	newEncoder func(w io.Writer) Encoder
	newDecoder func(r io.Reader) Decoder
}

func (c *customCodec) NewEncoder(w io.Writer) Encoder { return c.newEncoder(w) }
func (c *customCodec) NewDecoder(r io.Reader) Decoder { return c.newDecoder(r) }
func (c *customCodec) FileExtension() string          { return c.ext }

// NewCodec creates a custom Codec from user-provided encoder/decoder factories.
// This allows plugging in alternative JSON libraries (e.g. goccy/go-json,
// json-iterator, d3rty/json) or any other serialization format.
//
// Example with goccy/go-json:
//
//	import goccy "github.com/goccy/go-json"
//	codec := enginefile.NewCodec(".json",
//	    func(w io.Writer) enginefile.Encoder { return goccy.NewEncoder(w) },
//	    func(r io.Reader) enginefile.Decoder { return goccy.NewDecoder(r) },
//	)
func NewCodec(ext string, newEncoder func(w io.Writer) Encoder, newDecoder func(r io.Reader) Decoder) Codec {
	return &customCodec{
		ext:        ext,
		newEncoder: newEncoder,
		newDecoder: newDecoder,
	}
}
