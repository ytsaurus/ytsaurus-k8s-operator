package components

import (
	"context"
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"
)

type SyncStatus string

const (
	SyncStatusBlocked         SyncStatus = "Blocked"
	SyncStatusNeedFullUpdate  SyncStatus = "NeedFullUpdate"
	SyncStatusNeedLocalUpdate SyncStatus = "NeedLocalUpdate"
	SyncStatusPending         SyncStatus = "Pending"
	SyncStatusReady           SyncStatus = "Ready"
	SyncStatusUpdating        SyncStatus = "Updating"
)

type ComponentStatus struct {
	SyncStatus SyncStatus
	Message    string
}

func WaitingStatus(status SyncStatus, event string) ComponentStatus {
	return ComponentStatus{status, fmt.Sprintf("Wait for %s", event)}
}

func SimpleStatus(status SyncStatus) ComponentStatus {
	return ComponentStatus{status, string(status)}
}

type ServerComponent interface {
	Component
	GetPodsRemovedCondition() string
	ImageCorrespondsToSpec() bool
}

type ServerComponentBase struct {
	ComponentBase
	server Server
}

func (c *ServerComponentBase) GetPodsRemovedCondition() string {
	return c.labeller.GetPodsRemovedCondition()
}

func (c *ServerComponentBase) ImageCorrespondsToSpec() bool {
	return c.server.ImageCorrespondsToSpec()
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

			c.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
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
		c.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
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
	Status(ctx context.Context) ComponentStatus
	GetName() string
	GetLabel() string
	SetReadyCondition(status ComponentStatus)
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

func (c *ComponentBase) SetReadyCondition(status ComponentStatus) {
	ready := metav1.ConditionFalse
	if status.SyncStatus == SyncStatusReady {
		ready = metav1.ConditionTrue
	}
	c.ytsaurus.SetStatusCondition(metav1.Condition{
		Type:    fmt.Sprintf("%sReady", c.labeller.ComponentName),
		Status:  ready,
		Reason:  string(status.SyncStatus),
		Message: status.Message,
	})
}
