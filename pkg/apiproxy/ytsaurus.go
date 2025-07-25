package apiproxy

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

type Ytsaurus struct {
	ConditionManager
	apiProxy APIProxy
	ytsaurus *ytv1.Ytsaurus
}

func NewYtsaurus(
	ytsaurus *ytv1.Ytsaurus,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *Ytsaurus {
	return &Ytsaurus{
		ConditionManager: NewConditionManager(
			&ytsaurus.Status.Conditions,
			&ytsaurus.Status.UpdateStatus.Conditions,
		),
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

func (c *Ytsaurus) GetUpdatingComponents() []ytv1.Component {
	return c.ytsaurus.Status.UpdateStatus.UpdatingComponents
}

func (c *Ytsaurus) SetUpdatingComponents(canUpdate []ytv1.Component) {
	c.ytsaurus.Status.UpdateStatus.UpdatingComponents = canUpdate
	c.ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary = buildComponentsSummary(canUpdate)
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
	c.ytsaurus.Status.UpdateStatus.Conditions = make([]metav1.Condition, 0)
	c.ytsaurus.Status.UpdateStatus.TabletCellBundles = make([]ytv1.TabletCellBundleInfo, 0)
	c.ytsaurus.Status.UpdateStatus.MasterMonitoringPaths = make([]string, 0)
	c.SetUpdatingComponents(nil)
	return c.apiProxy.UpdateStatus(ctx)
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
	if c.ytsaurus.Status.ObservedGeneration == c.ytsaurus.Generation {
		return false
	}
	c.ytsaurus.Status.ObservedGeneration = c.ytsaurus.Generation
	return true
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

func buildComponentsSummary(components []ytv1.Component) string {
	if len(components) == 0 {
		return ""
	}
	var componentNames []string
	for _, comp := range components {
		// TODO: we better use labeller here after support deployment names in it.
		name := comp.Type.ShortName()
		if name == "" {
			name = strings.ToLower(string(comp.Type))
		}
		if comp.Name != "" {
			name += fmt.Sprintf("-%s", comp.Name)
		}
		componentNames = append(componentNames, name)
	}
	return "{" + strings.Join(componentNames, " ") + "}"
}
