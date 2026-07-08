package ypatch

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
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

func (t *CypressPatchTarget) SetNode(ctx context.Context, dst ypath.YPath, value any, options *yt.SetNodeOptions) error {
retry:
	err := t.Client.SetNode(ctx, dst, value, options)

	// Handle recursion when setting nested attributes without parents.
	// Workaround for bug: https://github.com/ytsaurus/ytsaurus/issues/1729
	// NOTE: Does not handle setting all attributes at once "/@", "/begin", "/end", etc.
	// NOTE: Will loose path attributes for Rich path argument due to ypath design flaw.
	if err != nil && yterrors.ContainsResolveError(err) && options != nil && options.Recursive && strings.Contains(string(dst.YPath()), "/@") {
		parent, child, err2 := ypath.Split(dst.YPath())
		if err2 != nil {
			return err2
		}
		// SetNode node/@parent/child value -> SetNode node/@parent {child=value}
		if !strings.HasPrefix(child, "/@") {
			dst = parent
			value = map[string]any{child[1:]: value}
			goto retry
		}
		// SetNode parent/@child value -> CreateNode parent {child=value}
		_, err = t.Client.CreateNode(ctx, parent, yt.NodeMap, &yt.CreateNodeOptions{
			Recursive:             true,
			Attributes:            map[string]any{child[2:]: value},
			TransactionOptions:    options.TransactionOptions,
			AccessTrackingOptions: options.AccessTrackingOptions,
			MutatingOptions:       options.MutatingOptions,
			PrerequisiteOptions:   options.PrerequisiteOptions,
		})
	}

	return err
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
				// FIXME: check non existence on add.
				err = t.SetNode(ctx, dst, op.Value, setOpts)
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
				if err = t.SetNode(ctx, dst, tmp, setOpts); err != nil {
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
