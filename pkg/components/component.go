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

type ComponentStatus struct {
	SyncStatus SyncStatus
	Message    string
}

func (s ComponentStatus) IsRunning() bool {
	return s.SyncStatus == SyncStatusReady || s.SyncStatus == SyncStatusNeedLocalUpdate
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
	GetType() consts.ComponentType

	// GetName returns component's name, which is used as an identifier in component management
	// and for mentioning in logs. For example: "Master", "DataNode<NameFromSpec>".
	GetName() string

	Fetch(ctx context.Context) error
	Sync(ctx context.Context, dry bool) (ComponentStatus, error)

	getAPIProxy() apiproxy.APIProxy
	getLabeller() *labeller.Labeller
}

type LocalComponent interface {
	Component

	getYtsaurus() *apiproxy.Ytsaurus
}

type LocalServerComponent interface {
	LocalComponent

	getServer() server
}

type RemoteServerComponent interface {
	Component
}

// Following structs are used as a base for implementing YTsaurus components objects.
type baseComponent struct {
	apiProxy apiproxy.APIProxy
	labeller *labeller.Labeller
}

func (c *baseComponent) GetType() consts.ComponentType {
	return c.labeller.ComponentType
}

func (c *baseComponent) GetName() string {
	return c.labeller.GetFullComponentName()
}

func (c *baseComponent) getAPIProxy() apiproxy.APIProxy {
	return c.apiProxy
}

func (c *baseComponent) getLabeller() *labeller.Labeller {
	return c.labeller
}

func newBaseComponent(
	apiProxy apiproxy.APIProxy,
	labeller *labeller.Labeller,
) baseComponent {
	return baseComponent{
		apiProxy: apiProxy,
		labeller: labeller,
	}
}

// localComponent is a base structs for components which have access to ytsaurus resource,
// but don't depend on server. Example: UI, Strawberry.
type localComponent struct {
	ytsaurus *apiproxy.Ytsaurus
	labeller *labeller.Labeller
}

func (c *localComponent) GetType() consts.ComponentType {
	return c.labeller.ComponentType
}

func (c *localComponent) GetName() string {
	return c.labeller.GetFullComponentName()
}

func (c *localComponent) getAPIProxy() apiproxy.APIProxy {
	return c.ytsaurus.APIProxy()
}

func (c *localComponent) getLabeller() *labeller.Labeller {
	return c.labeller
}

func (c *localComponent) getYtsaurus() *apiproxy.Ytsaurus {
	return c.ytsaurus
}

func newLocalComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
) localComponent {
	return localComponent{
		ytsaurus: ytsaurus,
		labeller: labeller,
	}
}

// localServerComponent is a base structs for components which have access to ytsaurus resource,
// and use server. Almost all components are based on this struct.
type localServerComponent struct {
	localComponent
	server server
}

func (c *localServerComponent) getServer() server {
	return c.server
}

func newLocalServerComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	server server,
) localServerComponent {
	return localServerComponent{
		localComponent: newLocalComponent(labeller, ytsaurus),
		server:         server,
	}
}

type remoteServerComponent struct {
	baseComponent
	server server
}

func newRemoteServerComponent(
	apiProxy apiproxy.APIProxy,
	labeller *labeller.Labeller,
	server server,
) remoteServerComponent {
	return remoteServerComponent{
		baseComponent: newBaseComponent(apiProxy, labeller),
		server:        server,
	}
}

func ServerNeedSync(s server, ytsaurus *apiproxy.Ytsaurus) bool {
	// FIXME(khlebnikov): Explain this logic and move into upper layer.
	return (s.configNeedsReload() && ytsaurus.IsUpdating()) || s.needBuild()
}

func GetReadyCondition(component Component, status ComponentStatus) metav1.Condition {
	ready := metav1.ConditionFalse
	if status.SyncStatus == SyncStatusReady {
		ready = metav1.ConditionTrue
	}
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReady", component.GetName()),
		Status:  ready,
		Reason:  string(status.SyncStatus),
		Message: status.Message,
	}
}
