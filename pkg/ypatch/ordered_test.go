package ypatch

import (
	"encoding/json"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/require"

	"go.ytsaurus.tech/yt/go/yson"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/canonize"
)

const (
	input = `{
    first1=1;
    nested2={
        y1=1;
        x2=2;
    };
    list3=[
        {
            j1=1;
            i2=2;
        };
    ];
    attrs4=<
        b1=1;
        a2=2;
    >
    {
        y1=1;
        x2=2;
    };
    last5=2;
}`

	override = `{
	attrs4=<b1=3>#;
    nested2={z3=3;y1=4};
	append6 = <>{};
    first1 = 4;
    last5=<attr=%true>{};
}`

	output = `{
    first1=4;
    nested2={
        y1=4;
        x2=2;
        z3=3;
    };
    list3=[
        {
            j1=1;
            i2=2;
        };
    ];
    attrs4=<
        b1=3;
        a2=2;
    >
    #;
    last5=<
        attr=%true;
    >
    {
    };
    append6=<
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
	canonize.AssertBlob(t, "input.yson", []byte(input))
	canonize.AssertStruct(t, "unmarshaled", m)

	require.Equal(t, []string{"first1", "nested2", "list3", "attrs4", "last5"}, m.Keys)

	nested, ok := m.GetMap("nested2")
	require.True(t, ok)
	require.Equal(t, []string{"y1", "x2"}, nested.Keys)

	marshaled, err := yson.MarshalFormat(&m, yson.FormatPretty)
	require.NoError(t, err)
	require.NoError(t, yson.Valid(marshaled))
	canonize.AssertBlob(t, "marshaled.yson", marshaled)
	require.Equal(t, []byte(input), marshaled)

	roundTripped := OrderedMap{}
	require.NoError(t, yson.Unmarshal(marshaled, &roundTripped))
	require.Equal(t, m.Keys, roundTripped.Keys)

	marshaledJSON, err := json.Marshal(&m)
	require.NoError(t, err)
	canonize.AssertBlob(t, "marshaled.json", marshaledJSON)

	mJSON := OrderedMap{}
	err = json.Unmarshal(marshaledJSON, &mJSON)
	require.NoError(t, err)
	canonize.AssertStruct(t, "unmarshaled.json", &mJSON)

	// TODO(khlebnikov): Fix YAML ordering. YAML generally is a mess.
	marshaledYAML, err := yaml.Marshal(&m)
	require.NoError(t, err)
	canonize.AssertBlob(t, "marshaled.yaml", marshaledYAML)

	mYAML := OrderedMap{}
	err = yaml.Unmarshal(marshaledYAML, &mYAML)
	require.NoError(t, err)
	canonize.AssertStruct(t, "unmarshaled.yaml", &mYAML)
}

func TestOrderedOverrideMergesContent(t *testing.T) {
	require.NoError(t, yson.Valid([]byte(input)))
	require.NoError(t, yson.Valid([]byte(override)))
	require.NoError(t, yson.Valid([]byte(output)))

	m := OrderedValue{}
	require.NoError(t, yson.Unmarshal([]byte(input), &m))
	canonize.AssertBlob(t, "input.yson", []byte(input))
	canonize.AssertStruct(t, "struct", m)

	canonize.AssertBlob(t, "override.yson", []byte(override))
	require.NoError(t, yson.Unmarshal([]byte(override), &m))
	canonize.AssertStruct(t, "merged", m)

	marshaled, err := yson.MarshalFormat(&m, yson.FormatPretty)
	require.NoError(t, err)
	require.NoError(t, yson.Valid(marshaled))
	canonize.AssertBlob(t, "marshaled.yson", marshaled)
	require.Equal(t, []byte(output), marshaled)
}
