package option_test

import (
	"encoding/json"
	"testing"

	"github.com/amberpixels/r3/internal/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoneAndSomeMethods(t *testing.T) {
	// Create a None option for int.
	optNone := option.None[int]()
	assert.True(t, optNone.None(), "Option should be None")
	assert.False(t, optNone.Some(), "Some() should be false for a None Option")
	assert.False(t, optNone.Some(42), "Some() should be false for a None Option")

	// Create a Some option using the helper.
	optSome := option.Some(42)
	assert.False(t, optSome.None(), "Option should not be None")
	assert.True(t, optSome.Some(), "Some() should return true when value is present")
	assert.True(t, optSome.Some(42), "Some(x) should return true when the value equals x")
	assert.False(t, optSome.Some(100), "Some(x) should return false when the value does not equal x")
}

func TestUnwrap(t *testing.T) {
	optSome := option.Some("hello")
	// Unwrap returns the contained value when present.
	assert.Equal(t, "hello", optSome.Unwrap(), "Unwrap should return the contained value")

	// Unwrap on a None should panic.
	optNone := option.None[string]()
	assert.Panics(t, func() { optNone.Unwrap() }, "Unwrap should panic on a None Option")
}

func TestJSONMarshalling(t *testing.T) {
	// Test marshalling of a Some option.
	optSome := option.Some(100)
	marshalledSome, err := json.Marshal(optSome)

	require.NoError(t, err, "Marshalling Some should not error")
	// Since optSome contains an integer, the JSON output should be that integer.
	assert.Equal(t, "100", string(marshalledSome))

	// Test marshalling of a None option.
	optNone := option.None[int]()
	marshalledNone, err := json.Marshal(&optNone) // should work with pointer as well
	require.NoError(t, err, "Marshalling None should not error")
	assert.Equal(t, "null", string(marshalledNone))

	// Test unmarshalling into a Some option.
	var optUnmarshalled option.Option[int]
	err = json.Unmarshal([]byte("123"), &optUnmarshalled)
	require.NoError(t, err, "Unmarshalling valid JSON should not error")
	assert.True(t, optUnmarshalled.Some(), "Option should have a value after unmarshalling")
	assert.True(t, optUnmarshalled.Some(123), "Unmarshalled value should be equal to 123")

	// Test unmarshalling null into an Option.
	err = json.Unmarshal([]byte("null"), &optUnmarshalled)
	require.NoError(t, err, "Unmarshalling JSON null should not error")
	assert.True(t, optUnmarshalled.None(), "Option should be None after unmarshalling null")
}

func TestUnmarshalText(t *testing.T) {
	// For testing UnmarshalText, we'll use Option[string] since string
	// works directly with json.Unmarshal in our fallback.
	var opt option.Option[string]

	// Test with non-null text.
	err := opt.UnmarshalText([]byte(`"test"`))
	require.NoError(t, err, "UnmarshalText with valid text should not error")
	assert.True(t, opt.Some(), "Option should have a value")
	assert.True(t, opt.Some("test"), "Option should contain the value 'test'")

	// Test with empty text.
	err = opt.UnmarshalText([]byte(""))
	require.NoError(t, err, "UnmarshalText with empty text should not error")
	assert.True(t, opt.None(), "Option should be None when given an empty string")

	// Test with the string "null" (case-insensitive).
	err = opt.UnmarshalText([]byte("NULL"))
	require.NoError(t, err, "UnmarshalText with 'NULL' should not error")
	assert.True(t, opt.None(), "Option should be None when given 'NULL'")
}
