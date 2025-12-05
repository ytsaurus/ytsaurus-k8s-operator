package components

import (
	"context"
	"fmt"
	"runtime"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
)

type SyncStatus string

const (
	SyncStatusUndefined  SyncStatus = ""           // Status is undecided or unknown
	SyncStatusPending    SyncStatus = "Pending"    // Reconciliation is possible
	SyncStatusReady      SyncStatus = "Ready"      // Running, Reconciliation is not required
	SyncStatusNeedUpdate SyncStatus = "NeedUpdate" // Running, Update is required
	SyncStatusBlocked    SyncStatus = "Blocked"    // Waiting, Reconciliation is blocked
	SyncStatusUpdating   SyncStatus = "Updating"   // Waiting, Update in progress, reconciliation is waiting
	SyncStatusError      SyncStatus = "Error"      // Waiting, Unhandled error have occurred
)

type ComponentStatus struct {
	SyncStatus SyncStatus
	Message    string
	Error      error
}

func (cs ComponentStatus) IsDefined() bool {
	return cs.SyncStatus != SyncStatusUndefined
}

func (cs ComponentStatus) IsPending() bool {
	return cs.SyncStatus == SyncStatusPending
}

func (cs ComponentStatus) IsRunning() bool {
	switch cs.SyncStatus {
	case SyncStatusReady, SyncStatusNeedUpdate:
		return true
	default:
		return false
	}
}

func (cs ComponentStatus) IsWaiting() bool {
	switch cs.SyncStatus {
	case SyncStatusBlocked, SyncStatusUpdating, SyncStatusError:
		return true
	default:
		return false
	}
}

func ComponentStatusUndefined() ComponentStatus {
	return ComponentStatus{SyncStatusUndefined, "Undefined", nil}
}

func ComponentStatusReady() ComponentStatus {
	return ComponentStatus{SyncStatusReady, "Ready", nil}
}

func ComponentStatusReadyAfter(message string) ComponentStatus {
	return ComponentStatus{SyncStatusReady, message, nil}
}

func ComponentStatusBlocked(message string) ComponentStatus {
	return ComponentStatus{SyncStatusBlocked, message, nil}
}

func ComponentStatusPending(message string) ComponentStatus {
	return ComponentStatus{SyncStatusPending, message, nil}
}

func ComponentStatusNeedUpdate(message string) ComponentStatus {
	return ComponentStatus{SyncStatusNeedUpdate, message, nil}
}

func ComponentStatusUpdating(message string) ComponentStatus {
	return ComponentStatus{SyncStatusUpdating, message, nil}
}

func ComponentStatusBlockedBy(cause string) ComponentStatus {
	return ComponentStatus{SyncStatusBlocked, fmt.Sprintf("Blocked by %s", cause), nil}
}

func ComponentStatusWaitingFor(event string) ComponentStatus {
	return ComponentStatus{SyncStatusPending, fmt.Sprintf("Waiting for %s", event), nil}
}

func ComponentStatusUpdateStep(part string) ComponentStatus {
	return ComponentStatus{SyncStatusUpdating, fmt.Sprintf("Updating %s", part), nil}
}

func ComponentStatusError(cause string, err error) ComponentStatus {
	return ComponentStatus{SyncStatusError, fmt.Sprintf("Error %s: %v", cause, err), err}
}

// TODO(khlebnikov): Remove after packing all errors.
func (cs ComponentStatus) Unpack() (ComponentStatus, error) {
	return cs, cs.Error
}

// TODO(khlebnikov): Replace this stub with status with meaningful message.
func SimpleStatus(status SyncStatus) ComponentStatus {
	_, file, line, _ := runtime.Caller(1)
	return ComponentStatus{status, fmt.Sprintf("%v: at %v:%v", status, file, line), nil}
}

type Component interface {
	Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
	Status(ctx context.Context) (ComponentStatus, error)
	GetFullName() string
	GetShortName() string
	GetType() consts.ComponentType
	SetReadyCondition(status ComponentStatus)
	IsReadyConditionTrue() bool

	GetLabeller() *labeller.Labeller

	GetCypressPatch() ypatch.PatchSet
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

func (c *localComponent) IsReadyConditionTrue() bool {
	return c.ytsaurus.IsStatusConditionTrue(c.labeller.GetReadyCondition())
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

// handleServerUpgrade decides what to do with server depending on update state.
// at UpdateStateWaitingForPodsRemoval -> Updating (pods removal).
// at UpdateStateWaitingForPodsCreation -> Pending (pods creation) - caller should create resources.
// otherwise -> Undefined.
func (c *localComponent) handleServerUpgrade(ctx context.Context, server server, dry bool) ComponentStatus {
	switch c.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForPodsRemoval:
		if !dry {
			err := removePods(ctx, server, c)
			if err != nil {
				return ComponentStatusError("pods removal", err)
			}
		}
		return ComponentStatusUpdateStep("pods removal")
	case ytv1.UpdateStateWaitingForPodsCreation:
		if !isPodsRemovingStarted(c) && c.IsReadyConditionTrue() {
			return ComponentStatusReady()
		}
		return ComponentStatusPending("pods creation")
	default:
		return ComponentStatusUndefined()
	}
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
