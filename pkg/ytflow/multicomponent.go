package ytflow

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/utils/strings/slices"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type multiComponent struct {
	name     ComponentName
	children map[string]component
	order    []string
}

func newMultiComponent(name ComponentName, children map[string]component) *multiComponent {
	// For the sake of tests stability we run everything in same order
	var order []string
	for n := range children {
		order = append(order, n)
	}
	sort.Strings(order)

	return &multiComponent{
		name:     name,
		children: children,
		order:    order,
	}
}

func (mc *multiComponent) GetName() string {
	return string(mc.name)
}
func (mc *multiComponent) Fetch(ctx context.Context) error {
	var errors []error
	for _, name := range mc.order {
		child := mc.children[name]
		if err := child.Fetch(ctx); err != nil {
			errWrapped := fmt.Errorf("fetch failed for %s: %w", name, err)
			errors = append(errors, errWrapped)
		}
	}

	if len(errors) == 0 {
		return nil
	}

	var multiErr = fmt.Errorf("fetch failed for %s", mc.name)
	for _, err := range errors {
		multiErr = fmt.Errorf("%w; %w", multiErr, err)
	}
	return multiErr
}
func (mc *multiComponent) Status(ctx context.Context) componentStatus {
	var statuses []string
	msg := ""
	for _, name := range mc.order {
		child := mc.children[name]
		status := child.Status(ctx)
		// TODO (l0kix2): remove string wrapper after golang 1.20 bc there is generic Index func.
		statuses = append(statuses, string(status.SyncStatus))
		msg += fmt.Sprintf("%s is %s %s;", name, status.SyncStatus, status.Message)
	}

	syncStatusesPriority := []string{
		string(components.SyncStatusPending),
		string(components.SyncStatusNeedLocalUpdate),
		string(components.SyncStatusBlocked),
		string(components.SyncStatusUpdating),
		string(components.SyncStatusReady),
	}

	sort.Slice(statuses, func(i, j int) bool {
		return slices.Index(syncStatusesPriority, statuses[i]) <
			slices.Index(syncStatusesPriority, statuses[j])
	})

	return componentStatus{
		SyncStatus: syncStatus(statuses[0]),
		Message:    msg,
	}
}
func (mc *multiComponent) Sync(ctx context.Context) error {
	var errors []error
	for _, name := range mc.order {
		child := mc.children[name]
		if err := child.Sync(ctx); err != nil {
			errWrapped := fmt.Errorf("sync failed for %s: %w", name, err)
			errors = append(errors, errWrapped)
		}
	}

	if len(errors) == 0 {
		return nil
	}

	var multiErr = fmt.Errorf("sync failed for %s", mc.name)
	for _, err := range errors {
		multiErr = fmt.Errorf("%w; %w", multiErr, err)
	}
	return multiErr
}
