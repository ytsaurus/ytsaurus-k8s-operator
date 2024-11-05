package apiproxy

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

type Ytsaurus struct {
	apiProxy[*ytv1.Ytsaurus]
}

var _ TypedAPIProxy[*ytv1.Ytsaurus] = &Ytsaurus{}

func NewYtsaurus(
	ytsaurus *ytv1.Ytsaurus,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *Ytsaurus {
	return &Ytsaurus{
		apiProxy: NewAPIProxy(ytsaurus, client, recorder, scheme),
	}
}

func (c *Ytsaurus) Spec() *ytv1.YtsaurusSpec {
	return &c.resource.Spec
}

func (c *Ytsaurus) GetCommonSpec() ytv1.CommonSpec {
	return c.resource.Spec.CommonSpec
}

func (c *Ytsaurus) GetClusterState() ytv1.ClusterState {
	return c.resource.Status.State
}

func (c *Ytsaurus) IsUpdating() bool {
	return c.GetClusterState() == ytv1.ClusterStateUpdating
}

func (c *Ytsaurus) GetUpdateState() ytv1.UpdateState {
	return c.resource.Status.UpdateStatus.State
}

func (c *Ytsaurus) GetLocalUpdatingComponents() []string {
	return c.resource.Status.UpdateStatus.Components
}

func (c *Ytsaurus) GetUpdateFlow() ytv1.UpdateFlow {
	return c.resource.Status.UpdateStatus.Flow
}

func (c *Ytsaurus) IsUpdateStatusConditionTrue(condition string) bool {
	return meta.IsStatusConditionTrue(c.resource.Status.UpdateStatus.Conditions, condition)
}

func (c *Ytsaurus) SetUpdateStatusCondition(ctx context.Context, condition metav1.Condition) {
	logger := log.FromContext(ctx)
	logger.Info("Setting update status condition", "condition", condition)
	meta.SetStatusCondition(&c.resource.Status.UpdateStatus.Conditions, condition)
	sortConditions(c.resource.Status.UpdateStatus.Conditions)
}

func (c *Ytsaurus) ClearUpdateStatus(ctx context.Context) error {
	c.resource.Status.UpdateStatus.Conditions = make([]metav1.Condition, 0)
	c.resource.Status.UpdateStatus.TabletCellBundles = make([]ytv1.TabletCellBundleInfo, 0)
	c.resource.Status.UpdateStatus.MasterMonitoringPaths = make([]string, 0)
	c.resource.Status.UpdateStatus.Components = nil
	c.resource.Status.UpdateStatus.Flow = ytv1.UpdateFlowNone
	return c.apiProxy.UpdateStatus(ctx)
}

func (c *Ytsaurus) LogUpdate(ctx context.Context, message string) {
	logger := log.FromContext(ctx)
	c.apiProxy.RecordNormal("Update", message)
	logger.Info(fmt.Sprintf("Ytsaurus update: %s", message))
}

func (c *Ytsaurus) SaveUpdatingClusterState(ctx context.Context, flow ytv1.UpdateFlow, components []string) error {
	logger := log.FromContext(ctx)
	c.resource.Status.State = ytv1.ClusterStateUpdating
	c.resource.Status.UpdateStatus.Flow = flow
	c.resource.Status.UpdateStatus.Components = components

	if err := c.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Ytsaurus cluster status")
		return err
	}

	return nil
}

func (c *Ytsaurus) SaveClusterState(ctx context.Context, clusterState ytv1.ClusterState) error {
	logger := log.FromContext(ctx)
	c.resource.Status.State = clusterState
	if err := c.apiProxy.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Ytsaurus cluster status")
		return err
	}

	return nil
}

// SyncObservedGeneration confirms that current generation was observed.
// Returns true if generation actually has been changed and status must be saved.
func (c *Ytsaurus) SyncObservedGeneration() bool {
	if c.resource.Status.ObservedGeneration == c.resource.Generation {
		return false
	}
	c.resource.Status.ObservedGeneration = c.resource.Generation
	return true
}

func (c *Ytsaurus) SaveUpdateState(ctx context.Context, updateState ytv1.UpdateState) error {
	logger := log.FromContext(ctx)
	c.resource.Status.UpdateStatus.State = updateState
	if err := c.apiProxy.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Ytsaurus update state")
		return err
	}
	return nil
}

func (c *Ytsaurus) SetStatusCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&c.resource.Status.Conditions, condition)
	sortConditions(c.resource.Status.Conditions)
}

func (c *Ytsaurus) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(c.resource.Status.Conditions, conditionType)
}

func (c *Ytsaurus) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(c.resource.Status.Conditions, conditionType)
}

func sortConditions(conditions []metav1.Condition) {
	slices.SortStableFunc(conditions, func(a, b metav1.Condition) int {
		statusOrder := []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown}
		if diff := cmp.Compare(slices.Index(statusOrder, a.Status), slices.Index(statusOrder, b.Status)); diff != 0 {
			return diff
		}
		return a.LastTransitionTime.Compare(b.LastTransitionTime.Time)
	})
}
