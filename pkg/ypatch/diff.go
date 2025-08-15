package ypatch

import (
	"fmt"

	"github.com/google/go-cmp/cmp"

	"go.ytsaurus.tech/yt/go/ypath"
)

type patchReporter struct {
	path  cmp.Path
	patch Patch
}

func (r *patchReporter) PushStep(step cmp.PathStep) {
	r.path = append(r.path, step)
}

func (r *patchReporter) PopStep() {
	r.path = r.path[:len(r.path)-1]
}

func (r *patchReporter) Report(rs cmp.Result) {
	if rs.Equal() {
		return
	}

	var path ypath.Path
	for _, step := range r.path {
		switch s := step.(type) {
		case cmp.StructField:
			path = path.Child(s.Name())
		case cmp.MapIndex:
			path = path.Child(fmt.Sprintf("%v", s.Key()))
		case cmp.SliceIndex:
			// FIXME: This is more complex.
			path = path.ListIndex(s.Key())
		}
	}

	before, after := r.path.Last().Values()
	if !before.IsValid() {
		r.patch = append(r.patch, PatchOperation{
			Op:    PatchOpAdd,
			Path:  path,
			Value: after.Interface(),
		})
	} else if !after.IsValid() {
		r.patch = append(r.patch, PatchOperation{
			Op:   PatchOpRemove,
			Path: path,
		})
	} else {
		r.patch = append(r.patch, PatchOperation{
			Op:    PatchOpReplace,
			Path:  path,
			Value: after.Interface(),
		})
	}
}

// BuildPatch constructs (mostly correct) patch to convert "before" into "after".
func BuildPatch(before, after any) Patch {
	reporter := &patchReporter{}
	if cmp.Equal(before, after, cmp.Reporter(reporter)) {
		return nil
	}
	return reporter.patch
}
