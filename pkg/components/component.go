package components

import (
	"context"
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func NewComponentStatus(status SyncStatus, message string) ComponentStatus {
	return ComponentStatus{status, message}
}

func WaitingStatus(status SyncStatus, event string) ComponentStatus {
	return ComponentStatus{status, fmt.Sprintf("Wait for %s", event)}
}

func SimpleStatus(status SyncStatus) ComponentStatus {
	return ComponentStatus{status, string(status)}
}

type Component interface {
	Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
	Status(ctx context.Context) ComponentStatus
	GetName() string
	GetLabel() string
	SetReadyCondition(status ComponentStatus)

	// TODO(nadya73): refactor it
	IsUpdatable() bool
}

type componentBase struct {
	labeller *labeller.Labeller
	ytsaurus *apiproxy.Ytsaurus
	cfgen    *ytconfig.Generator
}

func (c *componentBase) GetName() string {
	return c.labeller.ComponentName
}

func (c *componentBase) GetLabel() string {
	return c.labeller.ComponentLabel
}

func (c *componentBase) SetReadyCondition(status ComponentStatus) {
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
