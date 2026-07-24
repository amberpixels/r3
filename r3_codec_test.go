package r3_test

import (
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amberpixels/r3"
)

// codecModel exercises the built-in unix codecs across value/pointer fields and
// precisions. Type stays the domain type (time); the codec bridges to int.
type codecModel struct {
	ID        int64      `r3:"id,pk"`
	Name      string     `r3:"name"`
	StartedAt time.Time  `r3:"started_at,codec:unixtime"`
	ExpiresAt *time.Time `r3:"expires_at,codec:unixtime"`
	Millis    time.Time  `r3:"millis,codec:unixmilli"`
	Nanos     time.Time  `r3:"nanos,codec:unixnano"`
}

// codecFor returns the codec attached to the named attribute of T's schema.
func codecFor[T any](t *testing.T, name string) r3.Codec {
	t.Helper()
	attr, ok := r3.SchemaOf[T]().Lookup(name)
	require.True(t, ok, "attribute %q not found", name)
	require.NotNil(t, attr.Codec, "attribute %q has no codec", name)
	return attr.Codec
}

func TestSchemaOfAttachesCodec(t *testing.T) {
	schema := r3.SchemaOf[codecModel]()

	started, ok := schema.Lookup("started_at")
	require.True(t, ok)
	// Domain type stays "time" so callers filter with time.Time and validation
	// accepts it; the codec reports the stored type separately.
	assert.Equal(t, r3.TypeTime, started.Type)
	require.NotNil(t, started.Codec)
	assert.Equal(t, r3.TypeInt, started.Codec.Stored())

	// A plain field carries no codec.
	name, ok := schema.Lookup("name")
	require.True(t, ok)
	assert.Nil(t, name.Codec)
}

func TestUnixTimeCodecEncode(t *testing.T) {
	c := codecFor[codecModel](t, "started_at")

	// A concrete instant encodes to its unix seconds (zone-independent).
	tm := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	got, err := c.Encode(tm)
	require.NoError(t, err)
	assert.Equal(t, tm.Unix(), got)
	assert.IsType(t, int64(0), got)

	// A non-UTC input encodes to the same absolute stamp.
	loc := time.FixedZone("x", 3*60*60)
	got2, err := c.Encode(tm.In(loc))
	require.NoError(t, err)
	assert.Equal(t, tm.Unix(), got2)

	// Zero time and nil pointer both encode to NULL.
	gotZero, err := c.Encode(time.Time{})
	require.NoError(t, err)
	assert.Nil(t, gotZero)

	var nilPtr *time.Time
	gotNil, err := c.Encode(nilPtr)
	require.NoError(t, err)
	assert.Nil(t, gotNil)

	// A pointer to a real time encodes like the value.
	gotPtr, err := c.Encode(&tm)
	require.NoError(t, err)
	assert.Equal(t, tm.Unix(), gotPtr)

	// A non-time value is a usage error.
	_, err = c.Encode("nope")
	require.Error(t, err)
}

func TestUnixTimeCodecDecode(t *testing.T) {
	c := codecFor[codecModel](t, "started_at")
	timeType := reflect.TypeFor[time.Time]()
	tm := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

	// int64 (sql/sqlite), int (generic), float64 (json), and uint all decode.
	for _, stored := range []any{tm.Unix(), int(tm.Unix()), float64(tm.Unix()), uint64(tm.Unix())} {
		got, err := c.Decode(stored, timeType)
		require.NoErrorf(t, err, "stored=%T", stored)
		decoded, ok := got.(time.Time)
		require.Truef(t, ok, "stored=%T -> %T", stored, got)
		assert.Truef(t, decoded.Equal(tm), "stored=%T", stored)
		assert.Equal(t, time.UTC, decoded.Location(), "decode is UTC-canonical")
	}

	// String forms: numeric string and an RFC3339 fallback.
	gotNum, err := c.Decode(strconv.FormatInt(tm.Unix(), 10), timeType)
	require.NoError(t, err)
	assert.True(t, gotNum.(time.Time).Equal(tm))

	gotRFC, err := c.Decode(tm.Format(time.RFC3339), timeType)
	require.NoError(t, err)
	assert.True(t, gotRFC.(time.Time).Equal(tm))

	// []byte numeric (some drivers return raw bytes).
	gotBytes, err := c.Decode(fmt.Appendf(nil, "%d", tm.Unix()), timeType)
	require.NoError(t, err)
	assert.True(t, gotBytes.(time.Time).Equal(tm))

	// NULL decodes to the zero time for a value target.
	gotNull, err := c.Decode(nil, timeType)
	require.NoError(t, err)
	assert.True(t, gotNull.(time.Time).IsZero())

	// Unparseable string is an error.
	_, err = c.Decode("not-a-time", timeType)
	require.Error(t, err)
}

func TestUnixTimeCodecDecodePointerTarget(t *testing.T) {
	c := codecFor[codecModel](t, "expires_at")
	ptrType := reflect.TypeFor[*time.Time]()
	tm := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

	got, err := c.Decode(tm.Unix(), ptrType)
	require.NoError(t, err)
	p, ok := got.(*time.Time)
	require.True(t, ok)
	require.NotNil(t, p)
	assert.True(t, p.Equal(tm))

	// NULL decodes to a typed nil pointer.
	gotNull, err := c.Decode(nil, ptrType)
	require.NoError(t, err)
	np, ok := gotNull.(*time.Time)
	require.True(t, ok)
	assert.Nil(t, np)
}

func TestUnixTimeCodecPrecisionRoundTrip(t *testing.T) {
	// A sub-second instant round-trips exactly only at a fine-enough precision.
	tm := time.Date(2026, 7, 9, 12, 0, 0, 123456789, time.UTC)

	milli := codecFor[codecModel](t, "millis")
	ms, err := milli.Encode(tm)
	require.NoError(t, err)
	assert.Equal(t, tm.UnixMilli(), ms)
	backMs, err := milli.Decode(ms, reflect.TypeFor[time.Time]())
	require.NoError(t, err)
	assert.True(t, backMs.(time.Time).Equal(tm.Truncate(time.Millisecond)))

	nano := codecFor[codecModel](t, "nanos")
	ns, err := nano.Encode(tm)
	require.NoError(t, err)
	assert.Equal(t, tm.UnixNano(), ns)
	backNs, err := nano.Decode(ns, reflect.TypeFor[time.Time]())
	require.NoError(t, err)
	assert.True(t, backNs.(time.Time).Equal(tm), "nano precision is exact")
}

func TestSchemaOfUnknownCodecPanics(t *testing.T) {
	type badModel struct {
		ID int64     `r3:"id,pk"`
		At time.Time `r3:"at,codec:nope"`
	}
	err := recoverErr(func() { r3.SchemaOf[badModel]() })
	require.Error(t, err)
	require.ErrorIs(t, err, r3.ErrUnknownCodec)
}

func TestRegisterCodec(t *testing.T) {
	r3.RegisterCodec("test_upper", upperCodec{})

	type m struct {
		ID   int64  `r3:"id,pk"`
		Code string `r3:"code,codec:test_upper"`
	}
	c := codecFor[m](t, "code")
	assert.Equal(t, r3.TypeString, c.Stored())

	enc, err := c.Encode("abc")
	require.NoError(t, err)
	assert.Equal(t, "ABC", enc)

	assert.Panics(t, func() { r3.RegisterCodec("", upperCodec{}) })
	assert.Panics(t, func() { r3.RegisterCodec("x", nil) })
}

func TestRequireCodecSupport(t *testing.T) {
	// A schema with a codec panics with ErrCodecNotSupported.
	err := recoverErr(func() {
		r3.RequireCodecSupport(r3.SchemaOf[codecModel](), "r3/test")
	})
	require.Error(t, err)
	require.ErrorIs(t, err, r3.ErrCodecNotSupported)
	assert.Contains(t, err.Error(), "r3/test")

	// A codec-free schema and the zero schema both pass.
	type plain struct {
		ID   int64  `r3:"id,pk"`
		Name string `r3:"name"`
	}
	assert.NotPanics(t, func() { r3.RequireCodecSupport(r3.SchemaOf[plain](), "r3/test") })
	assert.NotPanics(t, func() { r3.RequireCodecSupport(r3.Schema{}, "r3/test") })
}

func TestEncodeFilterCodecs(t *testing.T) {
	schema := r3.SchemaOf[codecModel]()
	tm := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	lo := tm.Add(-time.Hour)
	hi := tm.Add(time.Hour)

	// Scalar comparison arg is encoded to the int stamp.
	out, err := r3.EncodeFilterCodecs(schema, r3.Filters{r3.Lt("started_at", tm)})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, tm.Unix(), out[0].Value)

	// Between pair.
	out, err = r3.EncodeFilterCodecs(schema, r3.Filters{r3.Between("started_at", lo, hi)})
	require.NoError(t, err)
	assert.Equal(t, []any{lo.Unix(), hi.Unix()}, out[0].Value)

	// In slice of time.Time.
	out, err = r3.EncodeFilterCodecs(schema, r3.Filters{r3.In("started_at", []time.Time{lo, hi})})
	require.NoError(t, err)
	assert.Equal(t, []any{lo.Unix(), hi.Unix()}, out[0].Value)

	// Nested And/Or is walked; a non-codec field is left untouched.
	out, err = r3.EncodeFilterCodecs(schema, r3.Filters{
		r3.And(r3.Lt("started_at", tm), r3.Eq("name", "x")),
	})
	require.NoError(t, err)
	require.Len(t, out[0].And, 2)
	assert.Equal(t, tm.Unix(), out[0].And[0].Value)
	assert.Equal(t, "x", out[0].And[1].Value)

	// Original filters are not mutated (immutability).
	orig := r3.Lt("started_at", tm)
	_, err = r3.EncodeFilterCodecs(schema, r3.Filters{orig})
	require.NoError(t, err)
	assert.Equal(t, tm, orig.Value, "input filter must be cloned, not mutated")

	// A schema without codecs is a no-op.
	type plain struct {
		ID int64  `r3:"id,pk"`
		At string `r3:"at"`
	}
	in := r3.Filters{r3.Eq("at", "v")}
	same, err := r3.EncodeFilterCodecs(r3.SchemaOf[plain](), in)
	require.NoError(t, err)
	assert.Equal(t, "v", same[0].Value)
}

func TestEncodeCursorCodecs(t *testing.T) {
	schema := r3.SchemaOf[codecModel]()
	tm := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

	// A raw time.Time cursor value encodes to the int stamp.
	out, err := r3.EncodeCursorCodecs(schema, r3.CursorValues{"started_at": tm, "name": "x"})
	require.NoError(t, err)
	assert.Equal(t, tm.Unix(), out["started_at"])
	assert.Equal(t, "x", out["name"], "non-codec key untouched")

	// The JSON-decoded form (RFC3339 string, as DecodeCursor yields) also encodes —
	// this is what makes a codec'd field usable as a real cursor key.
	out, err = r3.EncodeCursorCodecs(schema, r3.CursorValues{"started_at": tm.Format(time.RFC3339)})
	require.NoError(t, err)
	assert.Equal(t, tm.Unix(), out["started_at"])
}

// upperCodec is a trivial string codec used to prove the registry hook.
type upperCodec struct{}

func (upperCodec) Stored() r3.DataType { return r3.TypeString }

func (upperCodec) Encode(v any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("upperCodec: want string, got %T", v)
	}
	return upper(s), nil
}

func (upperCodec) Decode(stored any, _ reflect.Type) (any, error) {
	s, ok := stored.(string)
	if !ok {
		return nil, fmt.Errorf("upperCodec: want string, got %T", stored)
	}
	return s, nil
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - ('a' - 'A')
		}
	}
	return string(b)
}

func TestDecodeAggregateCodecs(t *testing.T) {
	schema := r3.SchemaOf[codecModel]()
	tm := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)

	q := r3.Query{
		GroupBy: r3.GroupBy("started_at"), // codec'd group-by column decodes
		Aggregates: r3.Aggregates{
			r3.AggMax("started_at", "last_started"),  // MIN/MAX over codec'd field decodes
			r3.AggMin("started_at", "first_started"), // ditto
			r3.AggSum("started_at", "sum_started"),   // SUM never decodes
			r3.AggAvg("started_at", "avg_started"),   // AVG never decodes
			r3.AggCount("n"),                         // COUNT never decodes
		},
	}
	rows := []r3.AggregateRow{{
		"started_at":    tm.Unix(),
		"last_started":  tm.Unix(),
		"first_started": tm.Add(-time.Hour).Unix(),
		"sum_started":   tm.Unix() * 2,
		"avg_started":   tm.Unix(),
		"n":             int64(2),
	}}

	require.NoError(t, r3.DecodeAggregateCodecs(schema, q, rows))

	// Group-by column and MIN/MAX aliases decode to time.Time.
	assertTimeEqual(t, tm, rows[0]["started_at"])
	assertTimeEqual(t, tm, rows[0]["last_started"])
	assertTimeEqual(t, tm.Add(-time.Hour), rows[0]["first_started"])
	// SUM/AVG/COUNT stay raw ints — decoding them would be nonsense.
	assert.Equal(t, tm.Unix()*2, rows[0]["sum_started"])
	assert.Equal(t, tm.Unix(), rows[0]["avg_started"])
	assert.Equal(t, int64(2), rows[0]["n"])
}

func TestDecodeAggregateCodecsNilAndNonCodec(t *testing.T) {
	schema := r3.SchemaOf[codecModel]()

	q := r3.Query{
		GroupBy: r3.GroupBy("name"), // non-codec'd group-by is untouched
		Aggregates: r3.Aggregates{
			r3.AggMax("started_at", "last_started"),
			r3.AggMax("id", "max_id"), // non-codec'd field: passes through
		},
	}
	rows := []r3.AggregateRow{{
		"name":         "launch",
		"last_started": nil, // MAX over an empty/all-NULL group
		"max_id":       int64(7),
	}}

	require.NoError(t, r3.DecodeAggregateCodecs(schema, q, rows))

	assert.Equal(t, "launch", rows[0]["name"], "non-codec group key untouched")
	assert.Nil(t, rows[0]["last_started"], "NULL extremum stays nil, not the zero time")
	assert.Equal(t, int64(7), rows[0]["max_id"], "non-codec aggregate untouched")

	// The nil result still reads back as ok=false through the accessor.
	_, ok := rows[0].Time("last_started")
	assert.False(t, ok)
}

func TestDecodeAggregateCodecsNoCodecSchemaIsNoop(t *testing.T) {
	type plain struct {
		ID        int64     `r3:"id,pk"`
		StartedAt time.Time `r3:"started_at"`
	}
	tm := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	q := r3.Query{Aggregates: r3.Aggregates{r3.AggMax("started_at", "last_started")}}
	rows := []r3.AggregateRow{{"last_started": tm.Unix()}}

	require.NoError(t, r3.DecodeAggregateCodecs(r3.SchemaOf[plain](), q, rows))
	assert.Equal(t, tm.Unix(), rows[0]["last_started"], "no codecs -> values untouched")

	// A fully zero schema is likewise a no-op.
	require.NoError(t, r3.DecodeAggregateCodecs(r3.Schema{}, q, rows))
	assert.Equal(t, tm.Unix(), rows[0]["last_started"])
}

// assertTimeEqual asserts that got is a time.Time equal (same instant) to want.
// It compares with time.Time.Equal, not reflect.DeepEqual, since equal instants
// can carry different internal representations.
func assertTimeEqual(t *testing.T, want time.Time, got any) {
	t.Helper()
	gt, ok := got.(time.Time)
	require.Truef(t, ok, "want time.Time, got %T", got)
	assert.Truef(t, want.Equal(gt), "want %s, got %s", want, gt)
}

// recoverErr runs f and returns any panicked error (nil if it did not panic).
func recoverErr(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", r)
			}
		}
	}()
	f()
	return err
}
