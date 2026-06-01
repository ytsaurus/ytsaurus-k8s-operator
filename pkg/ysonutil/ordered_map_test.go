package ysonutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.ytsaurus.tech/yt/go/yson"
)

func TestOrderedMapMarshalUnmarshal(t *testing.T) {
	m := NewOrderedMap()
	m.Set("z", int64(1))
	m.Set("a", "hello")
	m.Set("m", true)

	data, err := yson.Marshal(m)
	require.NoError(t, err)

	m2 := NewOrderedMap()
	require.NoError(t, yson.Unmarshal(data, m2))

	require.Equal(t, []string{"z", "a", "m"}, m2.Keys())

	v, ok := m2.Get("z")
	require.True(t, ok)
	require.Equal(t, int64(1), v)

	v, ok = m2.Get("a")
	require.True(t, ok)
	require.Equal(t, "hello", v)

	v, ok = m2.Get("m")
	require.True(t, ok)
	require.Equal(t, true, v)
}

func TestOrderedMapPreservesOrder(t *testing.T) {
	// Build a YSON map literal with known key order.
	raw := []byte(`{z=1;a=2;b=3;c=4}`)

	m := NewOrderedMap()
	require.NoError(t, yson.Unmarshal(raw, m))

	require.Equal(t, []string{"z", "a", "b", "c"}, m.Keys())
}

func TestOrderedMapRoundTrip(t *testing.T) {
	// Unmarshal → Marshal → Unmarshal; order must survive the round-trip.
	raw := []byte(`{c=3;b=2;a=1}`)

	m := NewOrderedMap()
	require.NoError(t, yson.Unmarshal(raw, m))

	encoded, err := yson.Marshal(m)
	require.NoError(t, err)

	m2 := NewOrderedMap()
	require.NoError(t, yson.Unmarshal(encoded, m2))

	require.Equal(t, m.Keys(), m2.Keys())
	for _, k := range m.Keys() {
		v1, _ := m.Get(k)
		v2, _ := m2.Get(k)
		require.Equal(t, v1, v2, "value mismatch for key %q", k)
	}
}

func TestOrderedMapNestedValues(t *testing.T) {
	raw := []byte(`{x={nested=42};y=[1;2;3]}`)

	m := NewOrderedMap()
	require.NoError(t, yson.Unmarshal(raw, m))

	require.Equal(t, []string{"x", "y"}, m.Keys())
}

func TestOrderedMapSetOverwrite(t *testing.T) {
	m := NewOrderedMap()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("a", 99) // overwrite; should not change key order

	require.Equal(t, []string{"a", "b"}, m.Keys())
	v, _ := m.Get("a")
	require.Equal(t, 99, v)
}

func TestOrderedMapEmpty(t *testing.T) {
	m := NewOrderedMap()
	data, err := yson.Marshal(m)
	require.NoError(t, err)

	m2 := NewOrderedMap()
	require.NoError(t, yson.Unmarshal(data, m2))
	require.Equal(t, 0, m2.Len())
}

func TestOrderedMapNotFound(t *testing.T) {
	m := NewOrderedMap()
	_, ok := m.Get("missing")
	require.False(t, ok)
}
