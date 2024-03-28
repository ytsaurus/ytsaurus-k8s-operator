package apiproxy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
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

func (c *Ytsaurus) GetCommonSpec() ytv1.CommonSpec {
	return c.GetResource().Spec.CommonSpec
}

func (c *Ytsaurus) GetClusterState() ytv1.ClusterState {
	return c.ytsaurus.Status.State
}

func (c *Ytsaurus) IsUpdating() bool {
	return c.GetClusterState() == ytv1.ClusterStateUpdating
}

func (c *Ytsaurus) GetUpdateState() ytv1.UpdateState {
	return c.ytsaurus.Status.UpdateStatus.State
}

func (c *Ytsaurus) GetLocalUpdatingComponents() []string {
	return c.ytsaurus.Status.UpdateStatus.Components
}

func (c *Ytsaurus) IsUpdateStatusConditionTrue(condition string) bool {
	return meta.IsStatusConditionTrue(c.ytsaurus.Status.UpdateStatus.Conditions, condition)
}

func (c *Ytsaurus) SetUpdateStatusCondition(ctx context.Context, condition metav1.Condition) {
	logger := log.FromContext(ctx)
	logger.Info("Setting update status condition", "condition", condition)
	meta.SetStatusCondition(&c.ytsaurus.Status.UpdateStatus.Conditions, condition)
}

func (c *Ytsaurus) ClearUpdateStatus(ctx context.Context) error {
	c.ytsaurus.Status.UpdateStatus.Conditions = make([]metav1.Condition, 0)
	c.ytsaurus.Status.UpdateStatus.TabletCellBundles = make([]ytv1.TabletCellBundleInfo, 0)
	c.ytsaurus.Status.UpdateStatus.MasterMonitoringPaths = make([]string, 0)
	c.ytsaurus.Status.UpdateStatus.Components = nil
	return c.apiProxy.UpdateStatus(ctx)
}

func (c *Ytsaurus) LogUpdate(ctx context.Context, message string) {
	logger := log.FromContext(ctx)
	c.apiProxy.RecordNormal("Update", message)
	logger.Info(fmt.Sprintf("Ytsaurus update: %s", message))
}

func (c *Ytsaurus) SaveUpdatingClusterState(ctx context.Context, components []string) error {
	logger := log.FromContext(ctx)
	c.ytsaurus.Status.State = ytv1.ClusterStateUpdating
	c.ytsaurus.Status.UpdateStatus.Components = components

	if err := c.apiProxy.UpdateStatus(ctx); err != nil {
		logger.Error(err, "unable to update Ytsaurus cluster status")
		return err
	}

	return nil
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
	meta.SetStatusCondition(&c.ytsaurus.Status.Conditions, condition)
}

func (c *Ytsaurus) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(c.ytsaurus.Status.Conditions, conditionType)
}

func (c *Ytsaurus) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(c.ytsaurus.Status.Conditions, conditionType)
}
