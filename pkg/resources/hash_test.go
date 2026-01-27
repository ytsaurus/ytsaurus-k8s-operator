package resources

import (
	"fmt"
	"testing"
)

type testStruct struct {
	Field string `json:"field,omitempty"`
}

var tests = []struct {
	items []any
	want  string
}{
	{[]any{}, ""},
	{[]any{nil}, ""},
	{[]any{nil, nil}, ""},
	{[]any{&testStruct{}}, "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"},
	{[]any{&testStruct{}, nil}, "44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a"},
	{[]any{nil, &testStruct{}}, "9f5365b124bc5da850f0bbcb4ae37d3798bf73071c7a731bf16bb7ecec84b40c"},
	{[]any{[]testStruct{}}, "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"},
	{[]any{[]testStruct{{}}}, "e10808d43975dc400731053386849f864f297e6c4f7519c380f3dbaf7067a840"},
	{[]any{&testStruct{}, &testStruct{}}, "b51f08b698d88d8027a935d9db649774949f5fb41a0c559bfee6a9a13225c72d"},
	{[]any{&testStruct{Field: "foo"}}, "14dee98c957bfb60b01da5c99b2644e50617d459201ca494490ee940b7b856c3"},
	{[]any{&testStruct{Field: "bar"}}, "85b352a75136e24d2ee709509d0e98756099aa770b54f945abebd7108470a88e"},
	{[]any{&testStruct{Field: "foo"}, &testStruct{Field: "bar"}}, "dc4676c1e5aa9397b45b2e3db9df503f4ffdbf3bcc83ad2af8a18cf90e6c0a69"},
	{[]any{[]testStruct{{Field: "foo"}, {Field: "bar"}}}, "7a56292f789cbc0112146d1b1991645408fdad15f911417d5ba4331d3c3f8827"},
}

func TestHash(t *testing.T) {
	for i, tt := range tests {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			hash, err := Hash(tt.items...)
			if err != nil {
				t.Fatalf("fail: %v", err)
			}
			if hash != tt.want {
				t.Fatalf("got: %v want: %v", hash, tt.want)
			}
		})
	}
}
