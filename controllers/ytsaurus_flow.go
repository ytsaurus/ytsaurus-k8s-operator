package controllers

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

type YtsaurusFlow struct {
	registry          *componentRegistry
	conditionsManager *stepsStateManager
	statuses          map[string]components.ComponentStatus
	steps             []flows.StepType
}

func newYtsaurusFlow(ytsaurusProxy *apiProxy.Ytsaurus) (*YtsaurusFlow, error) {
	return &YtsaurusFlow{
		registry:          buildComponentRegistry(ytsaurusProxy),
		conditionsManager: newStepsStateManager(ytsaurusProxy),
		statuses:          make(map[string]components.ComponentStatus),
	}, nil
}

// fetch should be called before build to collect components' statuses.
// If fetch fails it is most likely retryable error.
func (f *YtsaurusFlow) fetch(ctx context.Context) error {
	for _, comp := range f.registry.asSlice() {
		if err := comp.Fetch(ctx); err != nil {
			return err
		}
		f.statuses[comp.GetName()] = comp.Status(ctx)
	}
	return nil
}

// build should be called after fetch.
// It is separated from fetch because if build fails it is most likely
// configuration issue and no point of retrying.
func (f *YtsaurusFlow) build() error {
	steps, err := buildSteps(f.registry, f.statuses)
	if err != nil {
		return err
	}
	f.steps = steps
	return nil
}

func (f *YtsaurusFlow) advance(ctx context.Context) (flows.StepSyncStatus, error) {
	logger := log.FromContext(ctx)
	flow := flows.NewFlow(f.steps, f.conditionsManager, logger)
	return flow.Advance(ctx)
}
