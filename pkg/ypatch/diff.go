package ypatch

import (
	"fmt"

	"github.com/google/go-cmp/cmp"

	"go.ytsaurus.tech/yt/go/ypath"
)

type patchReporter struct {
	path     cmp.Path
	patch    Patch
	withTest bool
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
	before, after := r.path.Last().Values()

	var path ypath.Path
	for _, step := range r.path {
		switch s := step.(type) {
		case cmp.StructField:
			path = path.Child(s.Name())
		case cmp.MapIndex:
			path = path.Child(fmt.Sprintf("%v", s.Key()))
		case cmp.SliceIndex:
			indexBefore, indexAfter := s.SplitKeys()
			if after.IsValid() {
				path = path.ListIndex(indexAfter)
			} else {
				path = path.ListIndex(indexBefore)
			}
		}
	}

	if !before.IsValid() {
		r.patch = append(r.patch, Add(path, after.Interface()))
	} else if !after.IsValid() {
		if r.withTest {
			r.patch = append(r.patch, Test(path, before.Interface()))
		}
		r.patch = append(r.patch, Remove(path))
	} else {
		if r.withTest {
			r.patch = append(r.patch, Test(path, before.Interface()))
		}
		r.patch = append(r.patch, Replace(path, after.Interface()))
	}
}

type PatchOptions struct {
	WithTest bool
}

// BuildPatch constructs (mostly correct) patch to convert "before" into "after".
func BuildPatch(before, after any, options *PatchOptions) Patch {
	if options == nil {
		options = &PatchOptions{}
	}
	reporter := &patchReporter{
		withTest: options.WithTest,
	}
	if cmp.Equal(before, after, cmp.Reporter(reporter)) {
		return nil
	}
	return reporter.patch
}
