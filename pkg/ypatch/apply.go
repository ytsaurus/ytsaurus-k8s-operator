package ypatch

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
)

type PatchTarget interface {
	ApplyPatch(ctx context.Context, path ypath.Path, patch Patch) error
	ApplyPatchSet(ctx context.Context, path ypath.Path, patchSet PatchSet) error
}

type CypressPatchTarget struct {
	Client yt.CypressClient
	DryRun bool
}

func ypathIsAbsolute(path ypath.Path) bool {
	tokens, err := ypath.SplitTokens(string(path))
	return err == nil && tokens[0] == string(ypath.Root)
}

func (t *CypressPatchTarget) ApplyPatch(ctx context.Context, path ypath.Path, patch Patch) error {
	setOpts := &yt.SetNodeOptions{
		Recursive: true,  // Create parents
		Force:     false, // Modify only attributes and documents
	}
	removeOpts := &yt.RemoveNodeOptions{
		Recursive: true, // Delete sub-tree
		Force:     true, // Ignore missing
	}
	for i, op := range patch {
		var err error
		dst := path + op.Path
		switch op.Op {
		case PatchOpAdd, PatchOpReplace:
			if !t.DryRun {
				// FIXME: check non existence or add.
				err = t.Client.SetNode(ctx, dst, op.Value, setOpts)
			}
		case PatchOpCopy, PatchOpMove:
			src := op.From
			if !ypathIsAbsolute(src) {
				src = path + src
			}
			var tmp any
			if err = t.Client.GetNode(ctx, src, &tmp, nil); err != nil {
				break
			}
			if !t.DryRun {
				if err = t.Client.SetNode(ctx, dst, tmp, setOpts); err != nil {
					break
				}
				if op.Op == PatchOpMove {
					err = t.Client.RemoveNode(ctx, src, removeOpts)
				}
			}
		case PatchOpRemove:
			if !t.DryRun {
				err = t.Client.RemoveNode(ctx, dst, removeOpts)
			}
		case PatchOpTest:
			var tmp any
			if err = t.Client.GetNode(ctx, dst, &tmp, nil); err != nil {
				break
			}
			if delta := BuildPatch(tmp, op.Value, &PatchOptions{WithTest: true}); delta != nil {
				err = fmt.Errorf("test failed")
			}
		default:
			err = fmt.Errorf("unknown patch operation: %v", op.Op)
		}
		if err != nil {
			return fmt.Errorf("patch step %d failed for path %v: %w", i, dst, err)
		}
	}
	return nil
}

func (t *CypressPatchTarget) ApplyPatchSet(ctx context.Context, path ypath.Path, patchSet PatchSet) error {
	// NOTE: "<>//..." will be applied last.
	for _, patchPath := range slices.Sorted(maps.Keys(patchSet)) {
		if err := t.ApplyPatch(ctx, path+patchPath.YPath(), patchSet[patchPath]); err != nil {
			return err
		}
	}
	return nil
}
