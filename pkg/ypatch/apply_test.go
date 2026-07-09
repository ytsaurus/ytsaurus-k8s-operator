package ypatch

import (
	"context"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"

	mock_yt "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/mock"
)

func TestCypressPatchTargetApplyPatchCommonCases(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	client := mock_yt.NewMockClient(ctrl)
	target := CypressPatchTarget{Client: client}

	setOpts := &yt.SetNodeOptions{Recursive: true, Force: false}
	removeOpts := &yt.RemoveNodeOptions{Recursive: true, Force: true}
	copiedValue := "copied-value"
	movedValue := "moved-value"
	testValue := map[string]any{"ok": true}

	gomock.InOrder(
		client.EXPECT().SetNode(ctx, ypath.Path("//added"), "new-value", setOpts),
		client.EXPECT().SetNode(ctx, ypath.Path("//replaced"), map[string]any{"answer": 42}, setOpts),
		client.EXPECT().GetNode(ctx, ypath.Path("//source"), gomock.Any(), nil).DoAndReturn(nodeResult(copiedValue)),
		client.EXPECT().SetNode(ctx, ypath.Path("//copied"), copiedValue, setOpts),
		client.EXPECT().GetNode(ctx, ypath.Path("//move"), gomock.Any(), nil).DoAndReturn(nodeResult(movedValue)),
		client.EXPECT().SetNode(ctx, ypath.Path("//moved"), movedValue, setOpts),
		client.EXPECT().RemoveNode(ctx, ypath.Path("//move"), removeOpts),
		client.EXPECT().RemoveNode(ctx, ypath.Path("//remove"), removeOpts),
		client.EXPECT().GetNode(ctx, ypath.Path("//check"), gomock.Any(), nil).DoAndReturn(nodeResult(testValue)),
	)

	patch := Patch{
		Add("//added", "new-value"),
		Replace("//replaced", map[string]any{"answer": 42}),
		Copy("//copied", "//source"),
		Move("//moved", "//move"),
		Remove("//remove"),
		Test("//check", testValue),
	}

	if err := target.ApplyPatch(ctx, "", patch); err != nil {
		t.Fatalf("ApplyPatch() failed: %v", err)
	}
}

func TestCypressPatchTargetApplyPatchSetUsesSortedPaths(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	client := mock_yt.NewMockClient(ctrl)
	target := CypressPatchTarget{Client: client}

	setOpts := &yt.SetNodeOptions{Recursive: true, Force: false}
	gomock.InOrder(
		client.EXPECT().SetNode(ctx, ypath.Path("//a/value"), 1, setOpts),
		client.EXPECT().SetNode(ctx, ypath.Path("//b/value"), 2, setOpts),
	)

	patchSet := PatchSet{
		"//b": {Replace("/value", 2)},
		"//a": {Replace("/value", 1)},
	}

	if err := target.ApplyPatchSet(ctx, "", patchSet); err != nil {
		t.Fatalf("ApplyPatchSet() failed: %v", err)
	}
}

func TestCypressPatchTargetSetNodeWhenParentAttributeIsMissing(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	client := mock_yt.NewMockClient(ctrl)
	target := CypressPatchTarget{Client: client}

	setOpts := &yt.SetNodeOptions{Recursive: true, Force: false}
	value := map[string]any{"node_count": 10}
	value2 := map[string]any{"child": value}
	resolveErr := &yterrors.Error{Code: yterrors.CodeResolveError, Message: "missing parent attribute"}

	gomock.InOrder(
		client.EXPECT().SetNode(ctx, ypath.Path("//path/node/@parent/child"), value, setOpts).Return(resolveErr),
		client.EXPECT().SetNode(ctx, ypath.Path("//path/node/@parent"), value2, setOpts).Return(nil),
	)

	if err := target.ApplyPatch(ctx, "", Patch{Replace("//path/node/@parent/child", value)}); err != nil {
		t.Fatalf("ApplyPatch() failed: %v", err)
	}
}

func TestCypressPatchTargetSetNodeWhenNodeIsMissing(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	client := mock_yt.NewMockClient(ctrl)
	target := CypressPatchTarget{Client: client}

	setOpts := &yt.SetNodeOptions{Recursive: true, Force: false}
	value := map[string]any{"node_count": 10}
	value2 := map[string]any{"child": value}
	value3 := map[string]any{"parent": value2}
	resolveErr := &yterrors.Error{Code: yterrors.CodeResolveError, Message: "missing parent attribute"}

	gomock.InOrder(
		client.EXPECT().SetNode(ctx, ypath.Path("//path/node/@parent/child"), value, setOpts).Return(resolveErr),
		client.EXPECT().SetNode(ctx, ypath.Path("//path/node/@parent"), value2, setOpts).Return(resolveErr),
		client.EXPECT().CreateNode(
			ctx,
			ypath.Path("//path/node"),
			yt.NodeMap,
			&yt.CreateNodeOptions{Recursive: true, Attributes: value3},
		).Return(yt.NodeID{}, nil),
	)

	if err := target.ApplyPatch(ctx, "", Patch{Replace("//path/node/@parent/child", value)}); err != nil {
		t.Fatalf("ApplyPatch() failed: %v", err)
	}
}

func nodeResult(value any) func(context.Context, ypath.YPath, any, *yt.GetNodeOptions) error {
	return func(_ context.Context, _ ypath.YPath, result any, _ *yt.GetNodeOptions) error {
		reflect.ValueOf(result).Elem().Set(reflect.ValueOf(value))
		return nil
	}
}
