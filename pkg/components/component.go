package components

import (
	"context"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"
)

type SyncStatus string

const (
	SyncStatusBlocked    SyncStatus = "Blocked"
	SyncStatusNeedUpdate SyncStatus = "NeedUpdate"
	SyncStatusPending    SyncStatus = "Pending"
	SyncStatusReady      SyncStatus = "Ready"
	SyncStatusUpdating   SyncStatus = "Updating"
)

type ServerComponent interface {
	Component
	GetPodsRemovedCondition() string
	IsImageCorrespondsToSpec() bool
}

type ServerComponentBase struct {
	ComponentBase
	server Server
}

func (c *ServerComponentBase) GetPodsRemovedCondition() string {
	return c.labeller.GetPodsRemovedCondition()
}

func (c *ServerComponentBase) IsImageCorrespondsToSpec() bool {
	return c.server.IsImageCorrespondsToSpec()
}

func (c *ServerComponentBase) removePods(ctx context.Context, dry bool) error {
	var err error
	if !c.ytsaurus.IsUpdateStatusConditionTrue(c.labeller.GetPodsRemovingStartedCondition()) {
		if !dry {
			ss := c.server.RebuildStatefulSet()
			ss.Spec.Replicas = ptr.Int32(0)
			err = c.server.Sync(ctx)

			if err != nil {
				return err
			}

			err = c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    c.labeller.GetPodsRemovingStartedCondition(),
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Pods removing was started",
			})
		}
		return err
	}

	if !c.server.ArePodsRemoved() {
		return err
	}

	if !dry {
		err = c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    c.labeller.GetPodsRemovedCondition(),
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: "Pods removed",
		})
	}

	return err
}

type Component interface {
	Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
	Status(ctx context.Context) SyncStatus
	GetName() string
	GetLabel() string
}

type ComponentBase struct {
	labeller *labeller.Labeller
	ytsaurus *apiproxy.Ytsaurus
	cfgen    *ytconfig.Generator
}

func (c *ComponentBase) GetName() string {
	return c.labeller.ComponentName
}

func (c *ComponentBase) GetLabel() string {
	return c.labeller.ComponentLabel
}
