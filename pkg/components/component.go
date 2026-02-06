package components

import (
	"context"
	"fmt"

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

	// NeedSync returns true when component is need, able and permitted to sync resources.
	NeedSync() bool

	// NeedUpdate returns true when component needs instance update.
	NeedUpdate() bool

	Status(ctx context.Context) (ComponentStatus, error)
	GetFullName() string
	GetShortName() string
	GetType() consts.ComponentType
	SetReadyCondition(status ComponentStatus)

	GetLabeller() *labeller.Labeller

	GetCypressPatch() ypatch.PatchSet
	UpdatePreCheck(ctx context.Context) ComponentStatus
}

type PreheatSpecProvider interface {
	PreheatSpec() (images []string, nodeSelector map[string]string, tolerations []corev1.Toleration)
}
type component struct {
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

func (c *component) GetLabeller() *labeller.Labeller {
	return c.labeller
}

func (c *component) GetCypressPatch() ypatch.PatchSet {
	return nil
}

func (c *component) UpdatePreCheck(ctx context.Context) ComponentStatus {
	return ComponentStatusReady()
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

func (c *serverComponent) NeedUpdate() bool {
	return c.server.needUpdate()
}

func (c *serverComponent) PreheatSpec() (images []string, nodeSelector map[string]string, tolerations []corev1.Toleration) {
	if c.server == nil {
		return nil, nil, nil
	}
	return c.server.preheatSpec()
}

func (c *microserviceComponent) NeedSync() bool {
	return c.microservice.needSync()
}

func (c *microserviceComponent) NeedUpdate() bool {
	return c.microservice.needUpdate()
}
