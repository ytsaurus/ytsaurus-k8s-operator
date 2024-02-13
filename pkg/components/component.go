package components

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type SyncStatus string

// I suggest to change semantics of statuses:
// "Blocked" — nothing can be done by operator, human needed
// i.e. need to do full update but EnableFullUpdate = false or config generator failing with error
// "NeedFullUpdate" >> *this component* sync will require it's full rebuild (recreate pods and so on)
// (currently it means ytsaurus needs full update).
// maybe rename like "NeedSyncWithRebuild".
// "NeedLocalUpdate" >> "NeedSync" there are some changes, but it is not expected that component will be
// recreated (or maybe nothing bad can happen of such recreating)
// "Ready" — all good
// "Updating" — not ready, but nothing to do for operator
// "Pending" — currently this means not ready and waiting for some other component to be ready,
// I'm not sure if we need this distinction in practice.

const (
	SyncStatusBlocked         SyncStatus = "Blocked"
	SyncStatusNeedFullUpdate  SyncStatus = "NeedFullUpdate"
	SyncStatusNeedLocalUpdate SyncStatus = "NeedLocalUpdate"
	SyncStatusPending         SyncStatus = "Pending"
	SyncStatusReady           SyncStatus = "Ready"
	SyncStatusUpdating        SyncStatus = "Updating"
)

func IsRunningStatus(status SyncStatus) bool {
	return status == SyncStatusReady || status == SyncStatusNeedLocalUpdate || status == SyncStatusNeedFullUpdate
}

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
