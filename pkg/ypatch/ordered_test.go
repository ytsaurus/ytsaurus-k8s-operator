package ypatch

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.ytsaurus.tech/yt/go/yson"
)

const (
	input = `{
    first=1;
    nested={
        second=2;
        first=1;
    };
    list=[
        {
            beta=2;
            alpha=1;
        };
    ];
    attributed=<
        second=2;
        first=1;
    >
    {
        right=2;
        left=1;
    };
    last=2;
}`

	patch = `{
	attributed=<second=3>#;
    nested={third = 3};
	append = <>{};
    first = 4;
    last=<attr=%true>{};
}`

	output = `{
    first=4;
    nested={
        second=2;
        first=1;
        third=3;
    };
    list=[
        {
            beta=2;
            alpha=1;
        };
    ];
    attributed=<
        second=3;
        first=1;
    >
    #;
    last=<
        attr=%true;
    >
    {
    };
    append=<
    >
    {
    };
}`
)

func TestOrderedMapSetKeepsExistingKeyPosition(t *testing.T) {
	m := OrderedMap{}
	m.Set("first", 1)
	m.Set("second", 2)
	m.Set("first", 3)

	require.Equal(t, []string{"first", "second"}, m.Keys)
	value, ok := m.Get("first")
	require.True(t, ok)
	require.EqualValues(t, 3, value)
}

func TestOrderedMapMarshalUnmarshalPreservesOrder(t *testing.T) {
	m := OrderedMap{}
	require.NoError(t, yson.Unmarshal([]byte(input), &m))
	require.Equal(t, []string{"first", "nested", "list", "attributed", "last"}, m.Keys)

	nested, ok := m.GetMap("nested")
	require.True(t, ok)
	require.Equal(t, []string{"second", "first"}, nested.Keys)

	marshaled, err := yson.MarshalFormat(&m, yson.FormatPretty)
	require.NoError(t, err)
	require.NoError(t, yson.Valid(marshaled))
	require.Equal(t, []byte(input), marshaled)

	roundTripped := OrderedMap{}
	require.NoError(t, yson.Unmarshal(marshaled, &roundTripped))
	require.Equal(t, m.Keys, roundTripped.Keys)
}

func TestOrderedValueUnmarshalMergesContent(t *testing.T) {
	require.NoError(t, yson.Valid([]byte(input)))
	require.NoError(t, yson.Valid([]byte(patch)))
	require.NoError(t, yson.Valid([]byte(output)))

	m := OrderedValue{}
	require.NoError(t, yson.Unmarshal([]byte(input), &m))

	require.NoError(t, yson.Unmarshal([]byte(patch), &m))

	marshaled, err := yson.MarshalFormat(&m, yson.FormatPretty)
	require.NoError(t, err)
	require.NoError(t, yson.Valid(marshaled))
	require.Equal(t, []byte(output), marshaled)
}
