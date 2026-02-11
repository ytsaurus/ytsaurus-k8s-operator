package components

import (
	"context"
	"fmt"
	"runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
)

type SyncStatus string

const (
	SyncStatusUndefined  SyncStatus = ""           // Status is unknown
	SyncStatusReady      SyncStatus = "Ready"      // Running, reconciliation is not required
	SyncStatusBlocked    SyncStatus = "Blocked"    // Reconciliation is impossible
	SyncStatusPending    SyncStatus = "Pending"    // Reconciliation is possible
	SyncStatusNeedUpdate SyncStatus = "NeedUpdate" // Running, update is required
	SyncStatusUpdating   SyncStatus = "Updating"   // Update in progress
)

type ComponentStatus struct {
	Component  string // Set by SetStatus
	SyncStatus SyncStatus
	Message    string
}

func (cs ComponentStatus) IsUndefined() bool {
	return cs.SyncStatus == SyncStatusUndefined
}

func (cs ComponentStatus) IsReady() bool {
	return cs.SyncStatus == SyncStatusReady
}

func (cs ComponentStatus) IsNeedUpdate() bool {
	return cs.SyncStatus == SyncStatusNeedUpdate
}

func (cs ComponentStatus) IsRunning() bool {
	return cs.SyncStatus == SyncStatusReady || cs.SyncStatus == SyncStatusNeedUpdate
}

func (cs ComponentStatus) Blocker() ComponentStatus {
	return ComponentStatusSprintf(SyncStatusBlocked, "Blocked by %v: %v, %v", cs.Component, cs.SyncStatus, cs.Message)
}

func ComponentStatusSprintf(status SyncStatus, format string, a ...any) ComponentStatus {
	return ComponentStatus{"", status, fmt.Sprintf(format, a...)}
}

func ComponentStatusReady() ComponentStatus {
	return ComponentStatus{"", SyncStatusReady, "Ready"}
}

func ComponentStatusReadyAfter(format string, a ...any) ComponentStatus {
	return ComponentStatusSprintf(SyncStatusReady, format, a...)
}

func ComponentStatusBlocked(format string, a ...any) ComponentStatus {
	return ComponentStatusSprintf(SyncStatusBlocked, format, a...)
}

func ComponentStatusBlockedBy(format string, a ...any) ComponentStatus {
	return ComponentStatusBlocked("Blocked by "+format, a...)
}

func ComponentStatusPending(format string, a ...any) ComponentStatus {
	return ComponentStatusSprintf(SyncStatusPending, format, a...)
}

func ComponentStatusNeedUpdate(format string, a ...any) ComponentStatus {
	return ComponentStatusSprintf(SyncStatusNeedUpdate, format, a...)
}

func ComponentStatusUpdating(format string, a ...any) ComponentStatus {
	return ComponentStatusSprintf(SyncStatusUpdating, format, a...)
}

func ComponentStatusWaitingFor(format string, a ...any) ComponentStatus {
	return ComponentStatusSprintf(SyncStatusPending, "Waiting for "+format, a...)
}

func ComponentStatusUpdateStep(format string, a ...any) ComponentStatus {
	return ComponentStatusSprintf(SyncStatusUpdating, "Updating "+format, a...)
}

// TODO(khlebnikov): Replace this stub with status with meaningful message.
func SimpleStatus(status SyncStatus) ComponentStatus {
	_, file, line, _ := runtime.Caller(1)
	return ComponentStatusSprintf(status, "%v at %v:%v", status, file, line)
}

type Component interface {
	resources.Fetchable

	GetStatus() ComponentStatus
	SetStatus(status ComponentStatus)

	// NeedSync returns true when component is need, able and permitted to sync resources.
	NeedSync() bool

	// NeedUpdate returns SyncStatusNeedUpdate when component needs instance update.
	NeedUpdate() ComponentStatus

	Sync(ctx context.Context, dry bool) (ComponentStatus, error)

	GetFullName() string
	GetShortName() string
	GetType() consts.ComponentType
	GetComponent() ytv1.Component

	// Access component status saved as status condition in controller object.
	GetReadyCondition() ComponentStatus
	SetReadyCondition(status ComponentStatus)

	GetLabeller() *labeller.Labeller

	GetImageHeaterTarget() *ImageHeaterTarget

	GetCypressPatch() ypatch.PatchSet
	UpdatePreCheck(ctx context.Context) ComponentStatus
}

type ImageHeaterTarget struct {
	Images           map[string]string
	ImagePullSecrets []corev1.LocalObjectReference
	NodeSelector     map[string]string
	Tolerations      []corev1.Toleration
	NodeAffinity     *corev1.NodeAffinity
}

type component struct {
	status   ComponentStatus
	labeller *labeller.Labeller
	owner    apiproxy.APIProxy

	// TODO: Remove, owner must provide all required interfaces.
	ytsaurus *apiproxy.Ytsaurus
}

type serverComponent struct {
	component

	server server
}

type microserviceComponent struct {
	component

	microservice microservice
}

func newComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
) component {
	return component{
		labeller: labeller,
		owner:    ytsaurus,
		ytsaurus: ytsaurus,
	}
}

func newServerComponent(
	labeller *labeller.Labeller,
	owner apiproxy.APIProxy,
	server server,
) serverComponent {
	return serverComponent{
		component: component{
			labeller: labeller,
			owner:    owner,
		},
		server: server,
	}
}

func newLocalServerComponent(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	server server,
) serverComponent {
	return serverComponent{
		component: component{
			labeller: labeller,
			owner:    ytsaurus,
			ytsaurus: ytsaurus,
		},
		server: server,
	}
}

func (c *component) GetStatus() ComponentStatus {
	if c.status.SyncStatus == SyncStatusUndefined {
		return ComponentStatus{c.GetFullName(), SyncStatusUndefined, "BUG: component status undefined"}
	}
	return c.status
}

func (c *component) SetStatus(status ComponentStatus) {
	status.Component = c.GetFullName()
	c.status = status
}

// GetFullName returns component's name, which is used as an identifier in component management
// and for mentioning in logs.
// For example for master component name is "Master",
// For data node name looks like "DataNode<NameFromSpec>".
func (c *component) GetFullName() string {
	return c.labeller.GetFullComponentName()
}

func (c *component) GetShortName() string {
	return c.labeller.GetInstanceGroup()
}

func (c *component) GetType() consts.ComponentType {
	return c.labeller.ComponentType
}

func (c *component) GetComponent() ytv1.Component {
	return ytv1.Component{
		Type: c.GetType(),
		Name: c.GetShortName(),
	}
}

func (c *component) GetLabeller() *labeller.Labeller {
	return c.labeller
}

func (c *component) GetImageHeaterTarget() *ImageHeaterTarget {
	return nil
}

func (c *component) GetCypressPatch() ypatch.PatchSet {
	return nil
}

func (c *component) UpdatePreCheck(ctx context.Context) ComponentStatus {
	return ComponentStatusReady()
}

func (c *component) GetReadyCondition() ComponentStatus {
	cond := c.owner.GetStatusCondition(c.labeller.GetReadyCondition())
	switch {
	case cond == nil:
		return ComponentStatus{c.GetFullName(), SyncStatusUndefined, "Status unknown"}
	case cond.Status == metav1.ConditionUnknown:
		return ComponentStatus{c.GetFullName(), SyncStatusUndefined, cond.Message}
	case cond.Status == metav1.ConditionTrue:
		return ComponentStatus{c.GetFullName(), SyncStatusReady, cond.Message}
	default:
		return ComponentStatus{c.GetFullName(), SyncStatus(cond.Reason), cond.Message}
	}
}

func (c *component) SetReadyCondition(status ComponentStatus) {
	ready := metav1.ConditionFalse
	if status.SyncStatus == SyncStatusReady {
		ready = metav1.ConditionTrue
	}
	c.owner.SetStatusCondition(metav1.Condition{
		Type:    c.labeller.GetReadyCondition(),
		Status:  ready,
		Reason:  string(status.SyncStatus),
		Message: status.Message,
	})
}

func (c *component) IsUpdatingResources() bool {
	if c.ytsaurus == nil {
		return true
	}
	return c.ytsaurus.IsUpdating() &&
		c.ytsaurus.IsUpdatingComponent(c.GetType(), c.GetShortName()) &&
		c.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsCreation
}

func (c *serverComponent) Exists() bool {
	return c.server.Exists()
}

func (c *serverComponent) NeedSync() bool {
	return c.server.needSync(c.IsUpdatingResources())
}

func (c *serverComponent) NeedUpdate() ComponentStatus {
	return c.server.needUpdate()
}

func (c *serverComponent) GetImageHeaterTarget() *ImageHeaterTarget {
	return c.server.getImageHeaterTarget()
}

func (c *microserviceComponent) NeedSync() bool {
	return c.microservice.needSync()
}

func (c *microserviceComponent) NeedUpdate() ComponentStatus {
	return c.microservice.needUpdate()
}

func (c *microserviceComponent) GetImageHeaterTarget() *ImageHeaterTarget {
	return c.microservice.getImageHeaterTarget()
}
