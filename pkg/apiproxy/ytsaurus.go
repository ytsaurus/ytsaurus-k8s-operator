package apiproxy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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

func (c *Ytsaurus) IsUpdateStatusConditionFalse(condition string) bool {
	return meta.IsStatusConditionFalse(c.ytsaurus.Status.UpdateStatus.Conditions, condition)
}

func (c *Ytsaurus) SetUpdateStatusCondition(ctx context.Context, condition metav1.Condition) {
	logger := log.FromContext(ctx)
	logger.Info("Setting update status condition", "condition", condition)
	meta.SetStatusCondition(&c.ytsaurus.Status.UpdateStatus.Conditions, condition)
}

func (c *Ytsaurus) ClearUpdateStatus(ctx context.Context) error {
	return c.UpdateStatusRetryOnConflict(
		ctx,
		func(ytsaurusResource *ytv1.Ytsaurus) {
			ytsaurusResource.Status.UpdateStatus.Conditions = make([]metav1.Condition, 0)
			ytsaurusResource.Status.UpdateStatus.TabletCellBundles = make([]ytv1.TabletCellBundleInfo, 0)
			ytsaurusResource.Status.UpdateStatus.MasterMonitoringPaths = make([]string, 0)
			ytsaurusResource.Status.UpdateStatus.Components = nil
		},
	)
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

func (c *Ytsaurus) UpdateStatusRetryOnConflict(ctx context.Context, change func(ytsaurusResource *ytv1.Ytsaurus)) error {
	tryUpdate := func(ytsaurusResource *ytv1.Ytsaurus) error {
		change(ytsaurusResource)
		// You have to return err itself here (not wrapped inside another error)
		// so that RetryOnConflict can identify it correctly.
		return c.APIProxy().Client().Status().Update(ctx, ytsaurusResource)
	}

	err := tryUpdate(c.ytsaurus)
	if err == nil || !errors.IsConflict(err) {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the resource here; you need to refetch it on every try, since
		// if you got a conflict on the last update attempt then you need to get
		// the current version before making your own changes.
		name := c.ytsaurus.GetName()
		ytsaurusResource := ytv1.Ytsaurus{}
		if err = c.APIProxy().FetchObject(ctx, name, &ytsaurusResource); err != nil {
			return err
		}

		return tryUpdate(&ytsaurusResource)
	})
}
