package apiproxy

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/metrics"
)

type Ytsaurus struct {
	apiProxy APIProxy
	ytsaurus *ytv1.Ytsaurus
}

func NewYtsaurus(
	ytsaurus *ytv1.Ytsaurus,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *Ytsaurus {
	return &Ytsaurus{
		ytsaurus: ytsaurus,
		apiProxy: NewAPIProxy(ytsaurus, client, recorder, scheme),
	}
}

func (c *Ytsaurus) APIProxy() APIProxy {
	return c.apiProxy
}

func (c *Ytsaurus) GetResource() *ytv1.Ytsaurus {
	return c.ytsaurus
}

func (c *Ytsaurus) GetCommonSpec() *ytv1.CommonSpec {
	return &c.GetResource().Spec.CommonSpec
}

func (c *Ytsaurus) GetCommonPodSpec() *ytv1.PodSpec {
	return &c.GetResource().Spec.PodSpec
}

func (c *Ytsaurus) GetClusterFeatures() ytv1.ClusterFeatures {
	return ptr.Deref(c.GetCommonSpec().ClusterFeatures, ytv1.ClusterFeatures{})
}

func (c *Ytsaurus) GetClusterState() ytv1.ClusterState {
	return c.ytsaurus.Status.State
}

func (c *Ytsaurus) IsReadyToUpdate() bool {
	return c.GetClusterState() == ytv1.ClusterStateRunning
}

func (c *Ytsaurus) IsUpdating() bool {
	return c.GetClusterState() == ytv1.ClusterStateUpdating
}

func (c *Ytsaurus) GetUpdateState() ytv1.UpdateState {
	return c.ytsaurus.Status.UpdateStatus.State
}

func (c *Ytsaurus) GetUpdatingComponents() []ytv1.Component {
	return c.ytsaurus.Status.UpdateStatus.UpdatingComponents
}

func (c *Ytsaurus) IsUpdatingComponent(componentType consts.ComponentType, componentName string) bool {
	for _, component := range c.GetUpdatingComponents() {
		if component.Type == componentType && component.Name == componentName {
			return true
		}
	}
	return false
}

func (c *Ytsaurus) IsUpdateStatusConditionTrue(condition string) bool {
	return meta.IsStatusConditionTrue(c.ytsaurus.Status.UpdateStatus.Conditions, condition)
}

func (c *Ytsaurus) SetUpdateStatusCondition(ctx context.Context, condition metav1.Condition) {
	logger := log.FromContext(ctx)
	logger.Info("Setting update status condition", "condition", condition)
	meta.SetStatusCondition(&c.ytsaurus.Status.UpdateStatus.Conditions, condition)
	sortConditions(c.ytsaurus.Status.UpdateStatus.Conditions)
}

func (c *Ytsaurus) SetUpdatingComponents(canUpdate []ytv1.Component) {
	c.ytsaurus.Status.UpdateStatus.UpdatingComponents = canUpdate
	c.ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary = buildComponentsSummary(canUpdate)
}

// ShouldRunPreChecks is status-based and can flip to false after successful execution.
func (c *Ytsaurus) ShouldRunPreChecks(componentType consts.ComponentType, componentName string) bool {
	// is RunPreChecks enabled for this component at all?
	if !c.shouldEnablePreChecksFromSpec(componentType, componentName) {
		return false
	}

	// have we already completed pre-checks for this component?
	cond := meta.FindStatusCondition(
		c.ytsaurus.Status.UpdateStatus.Conditions,
		fmt.Sprintf("%sReady", componentName),
	)
	if cond != nil && cond.Status == metav1.ConditionTrue {
		// interpret as "already completed"
		return false
	}
	return true
}

func (c *Ytsaurus) shouldEnablePreChecksFromSpec(componentType consts.ComponentType, componentName string) bool {
	for _, selector := range c.ytsaurus.Spec.UpdatePlan {
		if selector.Component.Type == componentType &&
			(selector.Component.Name == "" || selector.Component.Name == componentName) {
			if selector.Strategy != nil {
				return ptr.Deref(selector.Strategy.RunPreChecks, true)
			}
		}
	}
	return true
}

func (c *Ytsaurus) SetBlockedComponents(components []ytv1.Component) bool {
	summary := buildComponentsSummary(components)
	if c.ytsaurus.Status.UpdateStatus.BlockedComponentsSummary == summary {
		return false
	}
	c.ytsaurus.Status.UpdateStatus.BlockedComponentsSummary = summary
	return true
}

func (c *Ytsaurus) ClearUpdateStatus(ctx context.Context) error {
	c.ytsaurus.Status.UpdateStatus.Conditions = keepImagesHeatedCondition(c.ytsaurus.Status.UpdateStatus.Conditions)
	c.ytsaurus.Status.UpdateStatus.TabletCellBundles = make([]ytv1.TabletCellBundleInfo, 0)
	c.SetUpdatingComponents(nil)
	return c.apiProxy.UpdateStatus(ctx)
}

// keepImagesHeatedCondition needed to keep image-heater hash for the next reconcile
// otherwise the controller would immediately think images need reheating
func keepImagesHeatedCondition(conditions []metav1.Condition) []metav1.Condition {
	condition := meta.FindStatusCondition(conditions, consts.ConditionImageHeaterReady)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		return make([]metav1.Condition, 0)
	}
	return []metav1.Condition{*condition}
}

func (c *Ytsaurus) LogUpdate(ctx context.Context, message string) {
	logger := log.FromContext(ctx)
	c.apiProxy.RecordNormal("Update", message)
	logger.Info(fmt.Sprintf("Ytsaurus update: %s", message))
}

func (c *Ytsaurus) SaveClusterState(ctx context.Context, clusterState ytv1.ClusterState) error {
	logger := log.FromContext(ctx)
	c.ytsaurus.Status.State = clusterState
	if err := c.apiProxy.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Ytsaurus cluster status")
		return err
	}

	return nil
}

// SyncObservedGeneration confirms that current generation was observed.
// Returns true if generation actually has been changed and status must be saved.
func (c *Ytsaurus) SyncObservedGeneration() bool {
	updated := false
	if c.ytsaurus.Status.ObservedGeneration != c.ytsaurus.Generation {
		c.ytsaurus.Status.ObservedGeneration = c.ytsaurus.Generation
		updated = true
	}
	if c.apiProxy.UpdateOperatorVersion(&c.ytsaurus.Status.Conditions) {
		updated = true
	}
	return updated
}

func (c *Ytsaurus) SaveUpdateState(ctx context.Context, updateState ytv1.UpdateState) error {
	logger := log.FromContext(ctx)
	c.ytsaurus.Status.UpdateStatus.State = updateState
	if err := c.apiProxy.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Ytsaurus update state")
		return err
	}
	return nil
}

func (c *Ytsaurus) SetStatusCondition(condition metav1.Condition) {
	condition.ObservedGeneration = c.ytsaurus.Generation
	meta.SetStatusCondition(&c.ytsaurus.Status.Conditions, condition)
	sortConditions(c.ytsaurus.Status.Conditions)
}

func (c *Ytsaurus) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(c.ytsaurus.Status.Conditions, conditionType)
}

func (c *Ytsaurus) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(c.ytsaurus.Status.Conditions, conditionType)
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

func buildComponentsSummary(components []ytv1.Component) string {
	if len(components) == 0 {
		return ""
	}
	var componentNames []string
	for _, comp := range components {
		name := getLabelerComponentName(comp)
		componentNames = append(componentNames, name)
	}
	return "{" + strings.Join(componentNames, " ") + "}"
}

func getComponentLabeler(component ytv1.Component) *labeller.Labeller {
	l := (&labeller.Labeller{}).ForComponent(component.Type, component.Name)
	return l
}

func getLabelerComponentName(component ytv1.Component) string {
	l := getComponentLabeler(component)
	l.UseShortNames = true
	return l.GetComponentShortName()
}

// UpdateOnDeleteComponentsSummary updates the UpdatingComponentsSummary with waiting time information
// for components in OnDelete mode
func (c *Ytsaurus) UpdateOnDeleteComponentsSummary(ctx context.Context, waitingOnDeleteConditionType string, includeWaitinDuration bool) {
	if c.GetResource().Status.UpdateStatus.State != ytv1.UpdateStateWaitingForPodsRemoval {
		return
	}

	var summaryParts []string

	// Find all components that are waiting on delete
	for _, component := range c.GetResource().Status.UpdateStatus.UpdatingComponents {
		condition := meta.FindStatusCondition(c.GetResource().Status.UpdateStatus.Conditions, waitingOnDeleteConditionType)

		if condition != nil && condition.Status == "True" {
			componentName := getLabelerComponentName(component)
			if includeWaitinDuration {
				waitingDuration := time.Since(condition.LastTransitionTime.Time)
				summaryParts = append(summaryParts, fmt.Sprintf("{%s (waiting %s)}",
					componentName,
					waitingDuration.Truncate(time.Second).String()))
				metrics.ObserveOnDeleteWait(c.GetResource().Name, c.GetResource().Namespace, componentName, &condition.LastTransitionTime)
			} else {
				summaryParts = append(summaryParts, fmt.Sprintf("{%s}", componentName))
				metrics.ObserveOnDeleteWait(c.GetResource().Name, c.GetResource().Namespace, componentName, nil)
			}
		}
	}

	if len(summaryParts) > 0 {
		c.GetResource().Status.UpdateStatus.UpdatingComponentsSummary = strings.Join(summaryParts, ", ")
	}
}

func (c *Ytsaurus) IsImageHeaterEnabled() bool {
	return ptr.Deref(c.GetClusterFeatures().EnableImageHeater, false)
}
