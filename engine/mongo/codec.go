package enginemongo

import (
	"context"
	"fmt"

	"github.com/amberpixels/r3"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// HasCodecs reports whether the type declares any value codec.
func (m *StructMeta) HasCodecs() bool { return len(m.Codecs) > 0 }

// codecSchema builds the minimal [r3.Schema] the core codec helpers
// ([r3.EncodeFilterCodecs], [r3.EncodeCursorCodecs], [r3.DecodeAggregateCodecs])
// consume: one attribute per codec'd field, named by its bson field name (the name
// callers reference in filters/sorts/group-by) and carrying the codec. A zero
// schema when no codecs are declared, which makes every helper a no-op.
func (m *StructMeta) codecSchema() r3.Schema {
	if len(m.Codecs) == 0 {
		return r3.Schema{}
	}
	attrs := make([]r3.Attribute, 0, len(m.Codecs))
	for name, c := range m.Codecs {
		attrs = append(attrs, r3.Attribute{Name: name, Codec: c})
	}
	return (r3.Schema{}).With(attrs...)
}

// encodeWriteDoc converts each codec'd field in doc to its stored form, in place.
// doc is a bson field name -> Go value map (as produced by ToBSONDoc).
func (m *StructMeta) encodeWriteDoc(doc map[string]any) error {
	for name, c := range m.Codecs {
		v, ok := doc[name]
		if !ok {
			continue
		}
		enc, err := c.Encode(v)
		if err != nil {
			return fmt.Errorf("mongo: encode codec field %q: %w", name, err)
		}
		doc[name] = enc
	}
	return nil
}

// encodeWriteValues converts codec'd values to stored form, in place, where fields
// and vals are aligned (as produced by FieldValuesForFields / FieldValues).
func (m *StructMeta) encodeWriteValues(fields []string, vals []any) error {
	if len(m.Codecs) == 0 {
		return nil
	}
	for i, name := range fields {
		c, ok := m.Codecs[name]
		if !ok {
			continue
		}
		enc, err := c.Encode(vals[i])
		if err != nil {
			return fmt.Errorf("mongo: encode codec field %q: %w", name, err)
		}
		vals[i] = enc
	}
	return nil
}

// decodeReadDoc converts each codec'd field in a scanned document back to its Go
// domain value, in place. A null (or absent) stored value is removed so the target
// struct field keeps its zero value rather than failing to decode a null.
func (m *StructMeta) decodeReadDoc(raw bson.M) error {
	for name, c := range m.Codecs {
		v, ok := raw[name]
		if !ok {
			continue
		}
		if v == nil {
			delete(raw, name)
			continue
		}
		// target=nil: decode to the codec's natural domain type; the re-marshal
		// below bridges to the struct field's exact shape (time.Time vs *time.Time).
		dec, err := c.Decode(v, nil)
		if err != nil {
			return fmt.Errorf("mongo: decode codec field %q: %w", name, err)
		}
		raw[name] = dec
	}
	return nil
}

// unmarshalWithCodecs decodes a scanned document into dst, applying value codecs.
// It decodes the codec'd fields to their domain values, then re-marshals so the
// driver's native decoder handles everything else (ObjectID, nested docs, the
// domain-typed codec fields). dst is a pointer to the target struct.
func unmarshalWithCodecs(meta *StructMeta, raw bson.M, dst any) error {
	if err := meta.decodeReadDoc(raw); err != nil {
		return err
	}
	b, err := bson.Marshal(raw)
	if err != nil {
		return fmt.Errorf("mongo: re-marshal decoded document: %w", err)
	}
	if err := bson.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("mongo: decode document: %w", err)
	}
	return nil
}

// decodeList drains cursor into a slice of T, applying value codecs when the type
// declares them (else a direct cursor.All).
func (r *BaseCRUD[T, ID]) decodeList(ctx context.Context, cursor *mongo.Cursor) ([]T, error) {
	if !r.Meta.HasCodecs() {
		var entities []T
		if err := cursor.All(ctx, &entities); err != nil {
			return nil, err
		}
		return entities, nil
	}

	var raws []bson.M
	if err := cursor.All(ctx, &raws); err != nil {
		return nil, err
	}
	if len(raws) == 0 {
		return nil, nil
	}
	entities := make([]T, len(raws))
	for i := range raws {
		if err := unmarshalWithCodecs(&r.Meta, raws[i], &entities[i]); err != nil {
			return nil, err
		}
	}
	return entities, nil
}

// decodeCursorInto decodes the cursor's current document into dst, applying value
// codecs when the type declares them (else a direct native decode).
func decodeCursorInto(cursor *mongo.Cursor, meta *StructMeta, dst any) error {
	if !meta.HasCodecs() {
		return cursor.Decode(dst)
	}
	var raw bson.M
	if err := cursor.Decode(&raw); err != nil {
		return err
	}
	return unmarshalWithCodecs(meta, raw, dst)
}
