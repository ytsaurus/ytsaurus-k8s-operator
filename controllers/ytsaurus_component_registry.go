package controllers

import (
	"context"
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type component interface {
	GetName() string
	Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
}

type statefulComponent interface {
	component
	// TODO (l0kix2): I suppose Status() should be able to return error
	Status(ctx context.Context) components.ComponentStatus
}

type componentRegistry struct {
	byName map[string]component
}

func newComponentRegistry() *componentRegistry {
	return &componentRegistry{
		byName: make(map[string]component),
	}
}

func (r *componentRegistry) add(comps ...component) {
	for _, comp := range comps {
		r.byName[comp.GetName()] = comp
	}
}

func (r *componentRegistry) asSlice() []component {
	var result []component
	for _, comp := range r.byName {
		result = append(result, comp)
	}
	return result
}

func (r *componentRegistry) getByName(name string) (component, bool) {
	val, ok := r.byName[name]
	return val, ok
}

// getListByType naively counts on condition that all component names starts with type.
func (r *componentRegistry) getListByType(compType consts.ComponentType) []component {
	var result []component
	for key, val := range r.byName {
		if strings.HasPrefix(key, string(compType)) {
			result = append(result, val)
		}
	}
	return result
}

func (r *componentRegistry) getByType(compType consts.ComponentType) (component, bool) {
	allOfType := r.getListByType(compType)
	if len(allOfType) == 0 {
		return nil, false
	}
	return allOfType[0], true
}
