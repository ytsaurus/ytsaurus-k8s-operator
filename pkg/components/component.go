package components

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
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

type labellable struct {
	labeller *labeller.Labeller
}

type ytsaurusAware struct {
	ytsaurus *apiproxy.Ytsaurus
}

type serverComponent struct {
	server server
}

type ytsaurusComponent struct {
	labellable
	ytsaurusAware
}

type ytsaurusServerComponent struct {
	ytsaurusComponent
	serverComponent
}

func (c *labellable) GetName() string {
	return c.labeller.ComponentName
}

func (c *labellable) GetLabel() string {
	return c.labeller.ComponentLabel
}

func newYtsaurusComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
) ytsaurusComponent {
	return ytsaurusComponent{
		labellable:    labellable{labeller: labeller},
		ytsaurusAware: ytsaurusAware{ytsaurus: ytsaurus},
	}
}

func (c *ytsaurusComponent) SetReadyCondition(status ComponentStatus) {
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

func newYtsaurusServerComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	server server,
) ytsaurusServerComponent {
	return ytsaurusServerComponent{
		ytsaurusComponent: ytsaurusComponent{
			labellable: labellable{
				labeller: labeller,
			},
			ytsaurusAware: ytsaurusAware{ytsaurus: ytsaurus},
		},
		serverComponent: serverComponent{
			server: server,
		},
	}
}

func (c *serverComponent) NeedSync() bool {
	return c.server.configNeedsReload() ||
		c.server.needSync()
}

func (c *ytsaurusServerComponent) NeedSync() bool {
	return (c.server.configNeedsReload() && c.ytsaurus.IsUpdating()) ||
		c.server.needSync()
}
