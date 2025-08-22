package ypatch

import (
	_ "embed"
	"reflect"
	"testing"

	"go.ytsaurus.tech/yt/go/yson"
)

//go:embed testdata/testcases.yson
var TestcasesYson []byte

type Testcase struct {
	Name     string `yson:"name"`
	Before   any    `yson:"before"`
	After    any    `yson:"after"`
	Patch    Patch  `yson:"patch"`
	AltPatch Patch  `yson:"alt_patch"`
	Result   string `yson:"result"`
}

func TestMakePatch(t *testing.T) {
	var testcases []Testcase
	err := yson.Unmarshal(TestcasesYson, &testcases)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range testcases {
		if tc.After == nil {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			patch := BuildPatch(tc.Before, tc.After, nil)
			if len(tc.AltPatch) != 0 && reflect.DeepEqual(tc.AltPatch, patch) {
				return
			}
			if !reflect.DeepEqual(tc.Patch, patch) {
				t.Fatalf("\npatch: %+v\nexpected: %+v\nalt: %+v\n", patch, tc.Patch, tc.AltPatch)
			}
		})
	}
}
