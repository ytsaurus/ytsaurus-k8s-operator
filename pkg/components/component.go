package components

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
)

type SyncStatus string

const (
	SyncStatusNeedLocalUpdate SyncStatus = "NeedLocalUpdate"
	SyncStatusPending         SyncStatus = "Pending"
	SyncStatusUpdating        SyncStatus = "Updating"

	SyncStatusNeedSync SyncStatus = "NeedSync"
	SyncStatusReady    SyncStatus = "Ready"
	SyncStatusBlocked  SyncStatus = "Blocked"
)

func IsRunningStatus(status SyncStatus) bool {
	return status == SyncStatusReady || status == SyncStatusNeedLocalUpdate
}

type ComponentStatus struct {
	SyncStatus SyncStatus
	Message    string
	Stage      string
}

func NewComponentStatus(status SyncStatus, message string) ComponentStatus {
	return ComponentStatus{SyncStatus: status, Message: message}
}

func WaitingStatus(status SyncStatus, event string) ComponentStatus {
	return ComponentStatus{SyncStatus: status, Message: fmt.Sprintf("Wait for %s", event)}
}

func SimpleStatus(status SyncStatus) ComponentStatus {
	return ComponentStatus{SyncStatus: status, Message: string(status)}
}

func NeedSyncStatus(message string) ComponentStatus {
	return ComponentStatus{SyncStatus: SyncStatusNeedSync, Message: message}
}

func UpdatingStatus(message string) ComponentStatus {
	return ComponentStatus{SyncStatus: SyncStatusUpdating, Message: message}
}

func ReadyStatus() ComponentStatus {
	return ComponentStatus{SyncStatus: SyncStatusReady}
}

type Component interface {
	Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
	Status(ctx context.Context) (ComponentStatus, error)
	GetName() string
	GetType() consts.ComponentType
	SetReadyCondition(status ComponentStatus)

	// TODO(nadya73): refactor it
	IsUpdatable() bool
}

type conditionManagerIface interface {
	SetTrue(context.Context, ConditionName) error
	SetTrueMsg(context.Context, ConditionName, string) error
	SetFalse(context.Context, ConditionName) error
	SetFalseMsg(context.Context, ConditionName, string) error
	Set(context.Context, ConditionName, bool) error
	SetMsg(context.Context, ConditionName, bool, string) error
	SetCond(context.Context, Condition) error
	SetCondMany(context.Context, ...Condition) error
	SetCondMsg(context.Context, Condition, string) error
	IsTrue(ConditionName) bool
	IsFalse(ConditionName) bool
	Is(condition Condition) bool
	IsSatisfied(condition Condition) bool
	IsNotSatisfied(condition Condition) bool
	All(conds ...Condition) bool
	Any(conds ...Condition) bool
}

type stateManagerInterface interface {
	SetTabletCellBundles(context.Context, []ytv1.TabletCellBundleInfo) error
	GetTabletCellBundles() []ytv1.TabletCellBundleInfo
	SetMasterMonitoringPaths(context.Context, []string) error
	GetMasterMonitoringPaths() []string
}

// Following structs are used as a base for implementing YTsaurus components objects.
// baseComponent is a base struct intendend for use in the simplest components and remote components
// (the ones that don't have access to the ytsaurus resource).
type baseComponent struct {
	labeller *labeller.Labeller
}

// GetName returns component's name, which is used as an identifier in component management
// and for mentioning in logs.
// For example for master component name is "Master",
// For data node name looks like "DataNode<NameFromSpec>".
func (c *baseComponent) GetName() string {
	return c.labeller.ComponentName
}

// localComponent is a base structs for components which have access to ytsaurus resource,
// but don't depend on server. Example: UI, Strawberry.
type localComponent struct {
	baseComponent
	ytsaurus *apiproxy.Ytsaurus

	// currently we have it in the component, but in the future we may
	// want to receive it from the outside of the component.
	condManager  conditionManagerIface
	stateManager stateManagerInterface
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
		condManager:   NewConditionManagerFromYtsaurus(ytsaurus),
		stateManager:  NewStateManagerFromYtsaurus(ytsaurus),
	}
}

func (c *localComponent) SetReadyCondition(status ComponentStatus) {
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

func (c *localServerComponent) NeedSync() bool {
	return LocalServerNeedSync(c.server, c.ytsaurus)
}

func LocalServerNeedSync(srv server, ytsaurus *apiproxy.Ytsaurus) bool {
	return (srv.configNeedsReload() && ytsaurus.IsUpdating()) ||
		srv.needBuild()
}
