package ypatch

import (
	"go.ytsaurus.tech/yt/go/ypath"
)

type PatchOp string

const (
	PatchOpAdd     PatchOp = "add"
	PatchOpCopy    PatchOp = "copy"
	PatchOpMove    PatchOp = "move"
	PatchOpRemove  PatchOp = "remove"
	PatchOpReplace PatchOp = "replace"
	PatchOpTest    PatchOp = "test"
)

type PatchOperation struct {
	Op    PatchOp    `json:"op" yson:"op"`
	Path  ypath.Path `json:"path" yson:"path"`
	From  ypath.Path `json:"from,omitempty" yson:"from,omitempty"`
	Value any        `json:"value,omitempty" yson:"value,omitempty"`
}

// Patch represents patch with RFC-6902 json patch semantics.
type Patch []PatchOperation

// PatchSet represents series of patches which are applied lexicographically.
// NOTE: "<>//..." is applied after all "//...".
type PatchSet map[ypath.Path]Patch

func (p PatchSet) AddPatch(path ypath.Path, patch Patch) {
	p[path] = append(p[path], patch...)
}

func (p PatchSet) AddPatchSet(patchSet PatchSet) {
	for path, patch := range patchSet {
		p.AddPatch(path, patch)
	}
}

func Add(path ypath.Path, value any) PatchOperation {
	return PatchOperation{
		Op:    PatchOpAdd,
		Path:  path,
		Value: value,
	}
}

func Copy(path, from ypath.Path) PatchOperation {
	return PatchOperation{
		Op:   PatchOpCopy,
		Path: path,
		From: from,
	}
}

func Move(path, from ypath.Path) PatchOperation {
	return PatchOperation{
		Op:   PatchOpMove,
		Path: path,
		From: from,
	}
}

func Remove(path ypath.Path) PatchOperation {
	return PatchOperation{
		Op:   PatchOpRemove,
		Path: path,
	}
}

func Replace(path ypath.Path, value any) PatchOperation {
	return PatchOperation{
		Op:    PatchOpReplace,
		Path:  path,
		Value: value,
	}
}

func ReplaceOrRemove[T any](path ypath.Path, value *T) PatchOperation {
	if value == nil {
		return Remove(path)
	} else {
		return Replace(path, value)
	}
}

func Test(path ypath.Path, value any) PatchOperation {
	return PatchOperation{
		Op:    PatchOpTest,
		Path:  path,
		Value: value,
	}
}
