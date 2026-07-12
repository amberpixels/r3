package r3

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Codec transforms a Go field value to and from its stored representation. It is
// declared once per attribute via the r3:"...,codec:<name>" struct tag, then
// applied uniformly by every codec-aware backend: on write (Go → stored), on read
// (stored → Go), and on filter/cursor arguments (a domain value is encoded before
// it reaches the query). The flagship is [time.Time] ⇄ unix int ("unixtime"), but
// the abstraction is general — money as cents, enums as ints, a struct as JSON.
//
// A Codec operates on plain Go values, so it is backend-neutral: an engine feeds
// it whatever it scanned (int64, int32, float64, []byte, string) and takes back a
// value to bind or marshal. Implementations MUST be pure and stateless — one
// instance is shared across all repositories and cached on the [Schema]. A codec
// never owns the physical column type (r3 does no DDL); the column must already be
// the type [Codec.Stored] reports.
type Codec interface {
	// Encode converts a Go field value into the value handed to the backend. It
	// returns (nil, nil) to store NULL — e.g. the zero time.Time or a nil pointer.
	Encode(goValue any) (any, error)

	// Decode converts a stored value back into the Go field of type target (a
	// pointer type for a nullable field), tolerating the representation variance
	// across backends (int64/int32/float64/[]byte/string) and mapping NULL to the
	// zero value or nil pointer. A nil target means no destination shape is known
	// (e.g. an [AggregateRow] has no struct field): decode to the codec's natural
	// domain type, which [DecodeAggregateCodecs] relies on.
	Decode(stored any, target reflect.Type) (any, error)

	// Stored reports the logical DataType of the stored form (e.g. [TypeInt] for
	// unixtime) — a hint for bind typing and cursor encoding, not a substitute for
	// the attribute's domain [DataType].
	Stored() DataType
}

// codecRegistry maps a codec name to its shared, stateless implementation, seeded
// with the built-ins and extended through [RegisterCodec]. codecRegistryMu guards
// it so a RegisterCodec during setup is safe against concurrent SchemaOf derivation.
var (
	codecRegistryMu sync.RWMutex
	codecRegistry   = map[string]Codec{
		codecUnixSeconds: unixTimeCodec{unit: unixSeconds},
		codecUnixMilli:   unixTimeCodec{unit: unixMillis},
		codecUnixMicro:   unixTimeCodec{unit: unixMicros},
		codecUnixNano:    unixTimeCodec{unit: unixNanos},
	}
)

// Built-in codec names. The precision variants are distinct names rather than an
// argument so tag parsing stays a simple prefix match (mirrors enum:).
const (
	codecUnixSeconds = "unixtime"  // time.Time ⇄ int64 unix seconds
	codecUnixMilli   = "unixmilli" // time.Time ⇄ int64 unix milliseconds
	codecUnixMicro   = "unixmicro" // time.Time ⇄ int64 unix microseconds
	codecUnixNano    = "unixnano"  // time.Time ⇄ int64 unix nanoseconds
)

// RegisterCodec registers a value codec under name for use in a struct tag
// (r3:"...,codec:<name>"). Call it during setup, before deriving schemas; it
// overwrites any existing registration and panics on an empty name or nil codec.
func RegisterCodec(name string, c Codec) {
	if name == "" {
		panic("r3: RegisterCodec called with an empty name")
	}
	if c == nil {
		panic("r3: RegisterCodec called with a nil codec")
	}
	codecRegistryMu.Lock()
	defer codecRegistryMu.Unlock()
	codecRegistry[name] = c
}

// lookupCodec resolves a registered codec by name.
func lookupCodec(name string) (Codec, bool) {
	codecRegistryMu.RLock()
	defer codecRegistryMu.RUnlock()
	c, ok := codecRegistry[name]
	return c, ok
}

// RequireCodecSupport panics if s declares any value codec, naming the attribute
// and backend. A backend that does not yet apply codecs calls this at construction
// so a declared codec fails loudly instead of silently storing the un-encoded
// value (corrupting data and breaking portability). backend is a short id for the
// message, e.g. "r3/mongo".
func RequireCodecSupport(s Schema, backend string) {
	for i := range s.attrs {
		if s.attrs[i].Codec != nil {
			panic(fmt.Errorf(
				"%w: attribute %q declares a codec that %s does not implement yet",
				ErrCodecNotSupported, s.attrs[i].Name, backend,
			))
		}
	}
}

// hasCodecs reports whether any attribute declares a value codec.
func (s Schema) hasCodecs() bool {
	for i := range s.attrs {
		if s.attrs[i].Codec != nil {
			return true
		}
	}
	return false
}

// EncodeFilterCodecs clones filters with every codec'd argument converted to
// stored form, so a domain value compares against stored column values (e.g.
// r3.Lt("started_at", someTime) against an int column). Scalar, In/NotIn slice,
// and Between-pair arguments are handled; operators carrying no comparable value
// (Exists, Like), non-codec'd fields, relationship filters, and dotted field names
// pass through. No-op when the schema declares no codecs.
func EncodeFilterCodecs(s Schema, filters Filters) (Filters, error) {
	if !s.hasCodecs() || len(filters) == 0 {
		return filters, nil
	}
	out := make(Filters, len(filters))
	for i, f := range filters {
		nf, err := encodeFilterSpec(s, f)
		if err != nil {
			return nil, err
		}
		out[i] = nf
	}
	return out, nil
}

// encodeFilterSpec returns f with any codec'd argument encoded, cloning only when
// it changes something so shared specs are never mutated.
func encodeFilterSpec(s Schema, f *FilterSpec) (*FilterSpec, error) {
	if f == nil {
		return nil, nil //nolint:nilnil // a nil spec passes through unchanged; not an error condition
	}
	if len(f.And) > 0 || len(f.Or) > 0 {
		and, err := EncodeFilterCodecs(s, f.And)
		if err != nil {
			return nil, err
		}
		or, err := EncodeFilterCodecs(s, f.Or)
		if err != nil {
			return nil, err
		}
		cp := *f
		cp.And, cp.Or = and, or
		return &cp, nil
	}
	if f.Field == nil || f.Relation != "" || !codecEncodableOp(f.Operator) {
		return f, nil
	}
	attr, ok := s.Lookup(f.Field.String())
	if !ok || attr.Codec == nil {
		return f, nil
	}
	encoded, err := encodeFilterArg(attr.Codec, f.Operator, f.Value)
	if err != nil {
		return nil, err
	}
	cp := *f
	cp.Value = encoded
	return &cp, nil
}

// encodeFilterArg encodes a single filter argument to stored form, respecting the
// operator's value shape: a Between-family pair, an In/NotIn slice, or a scalar.
func encodeFilterArg(c Codec, op FilterOperatorSpec, v any) (any, error) {
	switch {
	case isBetweenOp(op):
		lo, hi, err := ExtractBetweenBounds(v)
		if err != nil {
			return nil, err
		}
		elo, err := c.Encode(lo)
		if err != nil {
			return nil, err
		}
		ehi, err := c.Encode(hi)
		if err != nil {
			return nil, err
		}
		return []any{elo, ehi}, nil
	case op == OperatorIn || op == OperatorNotIn:
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
			return c.Encode(v)
		}
		out := make([]any, rv.Len())
		for i := range rv.Len() {
			e, err := c.Encode(rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			out[i] = e
		}
		return out, nil
	default:
		return c.Encode(v)
	}
}

// EncodeCursorCodecs clones the decoded cursor values with every codec'd key
// converted to stored form, so keyset predicates compare against stored column
// values. No-op when the schema declares no codecs.
func EncodeCursorCodecs(s Schema, values CursorValues) (CursorValues, error) {
	if !s.hasCodecs() || len(values) == 0 {
		return values, nil
	}
	out := make(CursorValues, len(values))
	for col, v := range values {
		attr, ok := s.Lookup(col)
		if !ok || attr.Codec == nil {
			out[col] = v
			continue
		}
		ev, err := attr.Codec.Encode(v)
		if err != nil {
			return nil, err
		}
		out[col] = ev
	}
	return out, nil
}

// DecodeAggregateCodecs decodes, in place, the [AggregateRow] values that still
// carry a codec'd attribute's domain meaning: a group-by column mapping to a
// codec'd attribute, and a MIN/MAX over a codec'd attribute (both are real field
// values). SUM/AVG/COUNT are never decoded — a sum of unix seconds is not a
// time.Time. A NULL (e.g. MIN/MAX over an empty group) is left nil so
// [AggregateRow.Time] still reports ok=false rather than the codec's zero value.
//
// No-op when the schema declares no codecs. Codec-aware [Aggregator] backends call
// it once before returning rows, so the "which columns decode" rule lives in one
// place.
func DecodeAggregateCodecs(s Schema, q Query, rows []AggregateRow) error {
	if !s.hasCodecs() || len(rows) == 0 {
		return nil
	}
	decoders := aggregateCodecDecoders(s, q)
	if len(decoders) == 0 {
		return nil
	}
	for _, row := range rows {
		for key, c := range decoders {
			v, ok := row[key]
			if !ok || v == nil { // absent or SQL NULL (e.g. MAX over an empty group)
				continue
			}
			// target=nil: an AggregateRow has no destination struct field, and
			// Attribute carries no reflect.Type — so decode to the codec's
			// natural domain type (e.g. a bare time.Time).
			decoded, err := c.Decode(v, nil)
			if err != nil {
				return err
			}
			row[key] = decoded
		}
	}
	return nil
}

// aggregateCodecDecoders maps each result key that must be decoded to the codec
// that decodes it: codec'd group-by columns and MIN/MAX aggregates over codec'd
// fields. Everything else is omitted, so the returned map is empty when nothing
// in the query touches a codec.
func aggregateCodecDecoders(s Schema, q Query) map[string]Codec {
	decoders := make(map[string]Codec)
	for _, g := range q.GroupBy {
		if attr, ok := s.Lookup(g.String()); ok && attr.Codec != nil {
			decoders[g.String()] = attr.Codec
		}
	}
	for _, a := range q.Aggregates {
		if a == nil || a.Field == nil {
			continue
		}
		if a.Func != AggregateMin && a.Func != AggregateMax {
			continue // SUM/AVG/COUNT/COUNT_DISTINCT never preserve the domain type
		}
		if attr, ok := s.Lookup(a.Field.String()); ok && attr.Codec != nil {
			decoders[a.Alias] = attr.Codec
		}
	}
	return decoders
}

// codecEncodableOp reports whether an operator carries a field-domain value that
// should be encoded to stored form. Exists (bool), Like/ILike (text patterns) do
// not compare a stored field value and are left as-is.
func codecEncodableOp(op FilterOperatorSpec) bool {
	switch op {
	case OperatorEq, OperatorNe, OperatorGt, OperatorGte, OperatorLt, OperatorLte,
		OperatorIn, OperatorNotIn,
		OperatorBetween, OperatorBetweenEx, OperatorBetweenExInc, OperatorBetweenIncEx:
		return true
	case OperatorUnspecified, OperatorExists, OperatorLike, OperatorNotLike, OperatorILike:
		return false
	default:
		return false
	}
}

// isBetweenOp reports whether an operator uses a two-element bound value.
func isBetweenOp(op FilterOperatorSpec) bool {
	return op == OperatorBetween || op == OperatorBetweenEx ||
		op == OperatorBetweenExInc || op == OperatorBetweenIncEx
}

// unixTimeCodec implements time.Time ⇄ int64 at a fixed precision — the inverse of
// GORM's schema.UnixSecondSerializer: here the Go field is the time.Time and the
// column is the int.
type unixTimeCodec struct{ unit unixUnit }

// unixUnit selects the precision of a unixTimeCodec.
type unixUnit int8

const (
	unixSeconds unixUnit = iota
	unixMillis
	unixMicros
	unixNanos
)

// Stored reports that a unix timestamp is stored as an integer.
func (unixTimeCodec) Stored() DataType { return TypeInt }

// Encode maps a time.Time / *time.Time to a UTC int64 at the configured precision;
// the zero time and a nil pointer encode to NULL. It also accepts a time's
// JSON-decoded form (RFC3339 or numeric string) so a codec'd field works as a
// cursor key — cursor tokens round-trip through JSON, turning a time into a string.
func (c unixTimeCodec) Encode(goValue any) (any, error) {
	t, ok := asTime(goValue)
	if !ok {
		s, isStr := asString(goValue)
		if !isStr {
			return nil, fmt.Errorf("r3: unixtime codec cannot encode %T (want time.Time)", goValue)
		}
		parsed, err := c.toTime(s)
		if err != nil {
			return nil, err
		}
		t = parsed
	}
	if t.IsZero() {
		return nil, nil //nolint:nilnil // (nil, nil) is the codec contract for NULL
	}
	return c.fromTime(t.UTC()), nil
}

// Decode maps a stored value back to a time.Time (or *time.Time when target is a
// pointer). It tolerates the numeric representation variance across backends and
// falls back to parsing an RFC3339 string, so it survives whatever a given
// backend returns for the column. A NULL decodes to the zero value / nil pointer.
func (c unixTimeCodec) Decode(stored any, target reflect.Type) (any, error) {
	if stored == nil {
		return zeroForTarget(target), nil
	}
	t, err := c.toTime(stored)
	if err != nil {
		return nil, err
	}
	return wrapForTarget(t.UTC(), target), nil
}

// fromTime extracts the int64 stamp for the codec's precision. Dedicated methods
// (not a raw nanosecond division) keep negative pre-epoch times exact.
func (c unixTimeCodec) fromTime(t time.Time) int64 {
	switch c.unit {
	case unixSeconds:
		return t.Unix()
	case unixMillis:
		return t.UnixMilli()
	case unixMicros:
		return t.UnixMicro()
	case unixNanos:
		return t.UnixNano()
	default:
		return t.Unix()
	}
}

// toTime reconstructs a time.Time from a stored value, tolerating int/uint/float
// numerics, their string/[]byte forms, and an RFC3339 fallback.
func (c unixTimeCodec) toTime(stored any) (time.Time, error) {
	if s, ok := asString(stored); ok {
		if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			return c.fromStamp(n), nil
		}
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(s)); err == nil {
			return parsed, nil
		}
		return time.Time{}, fmt.Errorf("r3: unixtime codec cannot decode string %q", s)
	}
	n, ok := asInt64(stored)
	if !ok {
		return time.Time{}, fmt.Errorf("r3: unixtime codec cannot decode %T", stored)
	}
	return c.fromStamp(n), nil
}

// fromStamp builds a time.Time from an int64 stamp at the codec's precision.
func (c unixTimeCodec) fromStamp(n int64) time.Time {
	switch c.unit {
	case unixSeconds:
		return time.Unix(n, 0)
	case unixMillis:
		return time.UnixMilli(n)
	case unixMicros:
		return time.UnixMicro(n)
	case unixNanos:
		return time.Unix(0, n)
	default:
		return time.Unix(n, 0)
	}
}

// asTime coerces a value to time.Time, unwrapping a pointer (nil pointer yields
// the zero time, which Encode maps to NULL).
func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case *time.Time:
		if t == nil {
			return time.Time{}, true
		}
		return *t, true
	default:
		return time.Time{}, false
	}
}

// asString reports whether v is a string or []byte and returns it as a string.
func asString(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return s, true
	case []byte:
		return string(s), true
	default:
		return "", false
	}
}

// asInt64 coerces any signed/unsigned integer or float value to int64.
func asInt64(v any) (int64, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(rv.Uint()), true //nolint:gosec // stored stamps are within int64 range
	case reflect.Float32, reflect.Float64:
		return int64(rv.Float()), true
	default:
		return 0, false
	}
}

// zeroForTarget returns the zero value for target (a nil pointer when target is a
// pointer, the zero struct/scalar otherwise). Used to map a decoded NULL back to
// the field's Go type.
func zeroForTarget(target reflect.Type) any {
	if target == nil {
		return time.Time{}
	}
	return reflect.Zero(target).Interface()
}

// wrapForTarget boxes t into target's shape: *time.Time when target is a pointer,
// time.Time otherwise.
func wrapForTarget(t time.Time, target reflect.Type) any {
	if target != nil && target.Kind() == reflect.Pointer {
		p := reflect.New(target.Elem())
		p.Elem().Set(reflect.ValueOf(t))
		return p.Interface()
	}
	return t
}
