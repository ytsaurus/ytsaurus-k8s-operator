package components

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type SyncStatus string

const (
	SyncStatusBlocked         SyncStatus = "Blocked"
	SyncStatusNeedLocalUpdate SyncStatus = "NeedLocalUpdate"
	SyncStatusPending         SyncStatus = "Pending"
	SyncStatusReady           SyncStatus = "Ready"
	SyncStatusUpdating        SyncStatus = "Updating"
)

func IsRunningStatus(status SyncStatus) bool {
	return status == SyncStatusReady || status == SyncStatusNeedLocalUpdate
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

func ComponentStatusBlockedBy(blocker Component) ComponentStatus {
	return ComponentStatus{SyncStatusBlocked, fmt.Sprintf("Waiting for %v", blocker.GetComponentName())}
}

func SimpleStatus(status SyncStatus) ComponentStatus {
	return ComponentStatus{status, string(status)}
}

type Component interface {
	Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
	Status(ctx context.Context) (ComponentStatus, error)
	GetComponentType() consts.ComponentType
	GetComponentName() consts.ComponentName
	GetInstanceGroup() string
	SetReadyCondition(status ComponentStatus)

	GetLabeller() *labeller.Labeller
}

// Following structs are used as a base for implementing YTsaurus components objects.
// baseComponent is a base struct intended for use in the simplest components and remote components
// (the ones that don't have access to the ytsaurus resource).
type baseComponent struct {
	labeller *labeller.Labeller
}

// GetComponentName returns component's name:"<ComponentType>[-<InstanceGroup>]".
// For example for master component name is "Master",
// For data node name looks like "DataNode-foo".
func (c *baseComponent) GetComponentName() consts.ComponentName {
	return c.labeller.GetComponentName()
}

func (c *baseComponent) GetInstanceGroup() string {
	return c.labeller.GetInstanceGroup()
}

func (c *baseComponent) GetComponentType() consts.ComponentType {
	return c.labeller.ComponentType
}

func (c *baseComponent) GetLabeller() *labeller.Labeller {
	return c.labeller
}

// localComponent is a base structs for components which have access to ytsaurus resource,
// but don't depend on server. Example: UI, Strawberry.
type localComponent struct {
	baseComponent
	ytsaurus *apiproxy.Ytsaurus
}

// localServerComponent is a base structs for components which have access to ytsaurus resource,
// and use server. Almost all components are based on this struct.
type localServerComponent struct {
	localComponent
	server server
}

func newLocalComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
) localComponent {
	return localComponent{
		baseComponent: baseComponent{labeller: labeller},
		ytsaurus:      ytsaurus,
	}
}

func (c *localComponent) SetReadyCondition(status ComponentStatus) {
	ready := metav1.ConditionFalse
	if status.SyncStatus == SyncStatusReady {
		ready = metav1.ConditionTrue
	}
	c.ytsaurus.SetStatusCondition(metav1.Condition{
		Type:    c.labeller.GetReadyCondition(),
		Status:  ready,
		Reason:  string(status.SyncStatus),
		Message: status.Message,
	})
}

func newLocalServerComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	server server,
) localServerComponent {
	return localServerComponent{
		localComponent: localComponent{
			baseComponent: baseComponent{
				labeller: labeller,
			},
			ytsaurus: ytsaurus,
		},
		server: server,
	}
}

func (c *localServerComponent) NeedSync() bool {
	return LocalServerNeedSync(c.server, c.ytsaurus)
}

func LocalServerNeedSync(srv server, ytsaurus *apiproxy.Ytsaurus) bool {
	return (srv.configNeedsReload() && ytsaurus.IsUpdating()) ||
		srv.needBuild()
}
