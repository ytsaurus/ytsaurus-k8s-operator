package components

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
)

type SyncStatus string

const (
	SyncStatusReady      SyncStatus = "Ready"      // Running, reconciliation is not required
	SyncStatusBlocked    SyncStatus = "Blocked"    // Reconciliation is impossible
	SyncStatusPending    SyncStatus = "Pending"    // Reconciliation is possible
	SyncStatusNeedUpdate SyncStatus = "NeedUpdate" // Running, update is required
	SyncStatusUpdating   SyncStatus = "Updating"   // Update in progress
)

type ComponentStatus struct {
	SyncStatus SyncStatus
	Message    string
}

func (cs ComponentStatus) IsRunning() bool {
	return cs.SyncStatus == SyncStatusReady || cs.SyncStatus == SyncStatusNeedUpdate
}

func ComponentStatusReady() ComponentStatus {
	return ComponentStatus{SyncStatusReady, "Ready"}
}

func ComponentStatusReadyAfter(message string) ComponentStatus {
	return ComponentStatus{SyncStatusReady, message}
}

func ComponentStatusBlocked(message string) ComponentStatus {
	return ComponentStatus{SyncStatusBlocked, message}
}

func ComponentStatusPending(message string) ComponentStatus {
	return ComponentStatus{SyncStatusPending, message}
}

func ComponentStatusNeedUpdate(message string) ComponentStatus {
	return ComponentStatus{SyncStatusNeedUpdate, message}
}

func ComponentStatusUpdating(message string) ComponentStatus {
	return ComponentStatus{SyncStatusUpdating, message}
}

func ComponentStatusBlockedBy(cause string) ComponentStatus {
	return ComponentStatus{SyncStatusBlocked, fmt.Sprintf("Blocked by %s", cause)}
}

func ComponentStatusWaitingFor(event string) ComponentStatus {
	return ComponentStatus{SyncStatusPending, fmt.Sprintf("Waiting for %s", event)}
}

func ComponentStatusUpdateStep(part string) ComponentStatus {
	return ComponentStatus{SyncStatusUpdating, fmt.Sprintf("Updating %s", part)}
}

// TODO(khlebnikov): Replace this stub with status with meaningful message.
func SimpleStatus(status SyncStatus) ComponentStatus {
	return ComponentStatus{status, string(status)}
}

type Component interface {
	resources.Fetchable
	resources.Syncable

	Status(ctx context.Context) (ComponentStatus, error)
	GetFullName() string
	GetShortName() string
	GetType() consts.ComponentType
	SetReadyCondition(status ComponentStatus)

	GetLabeller() *labeller.Labeller

	GetCypressPatch() ypatch.PatchSet
	UpdatePreCheck(ctx context.Context) ComponentStatus
}

// Following structs are used as a base for implementing YTsaurus components objects.
// baseComponent is a base struct intended for use in the simplest components and remote components
// (the ones that don't have access to the ytsaurus resource).
type baseComponent struct {
	labeller *labeller.Labeller
}

// GetFullName returns component's name, which is used as an identifier in component management
// and for mentioning in logs.
// For example for master component name is "Master",
// For data node name looks like "DataNode<NameFromSpec>".
func (c *baseComponent) GetFullName() string {
	return c.labeller.GetFullComponentName()
}

func (c *baseComponent) GetShortName() string {
	return c.labeller.GetInstanceGroup()
}

func (c *baseComponent) GetType() consts.ComponentType {
	return c.labeller.ComponentType
}

func (c *baseComponent) GetLabeller() *labeller.Labeller {
	return c.labeller
}

func (c *baseComponent) GetCypressPatch() ypatch.PatchSet {
	return nil
}

func (c *baseComponent) UpdatePreCheck(ctx context.Context) ComponentStatus {
	return ComponentStatusReady()
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

func (c *localServerComponent) Exists() bool {
	return c.server.Exists()
}

func (c *localServerComponent) NeedSync() bool {
	return LocalServerNeedSync(c.server, c.ytsaurus)
}

func LocalServerNeedSync(srv server, ytsaurus *apiproxy.Ytsaurus) bool {
	return (srv.configNeedsReload() && ytsaurus.IsUpdating()) ||
		srv.needBuild()
}
