package controllers

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type ComponentManager struct {
	ytsaurus      *apiProxy.Ytsaurus
	allComponents []components.Component
	status        ComponentManagerStatus
}

type ComponentManagerStatus struct {
	needSync           bool
	needInit           bool
	needUpdate         []components.Component
	allReadyOrUpdating bool
}

func NewComponentManager(
	ctx context.Context,
	ytsaurus *apiProxy.Ytsaurus,
) (*ComponentManager, error) {
	logger := log.FromContext(ctx)
	resource := ytsaurus.GetResource()

	clusterDomain := getClusterDomain(ytsaurus.APIProxy().Client())
	cfgen := ytconfig.NewGenerator(resource, clusterDomain)
	nodeCfgGen := &cfgen.NodeGenerator

	d := components.NewDiscovery(cfgen, ytsaurus)
	m := components.NewMaster(cfgen, ytsaurus)
	var hps []components.Component
	for _, hpSpec := range ytsaurus.GetResource().Spec.HTTPProxies {
		hps = append(hps, components.NewHTTPProxy(cfgen, ytsaurus, m, hpSpec))
	}
	yc := components.NewYtsaurusClient(cfgen, ytsaurus, hps[0])

	var dnds []components.Component
	if len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range ytsaurus.GetResource().Spec.DataNodes {
			dnds = append(dnds, components.NewDataNode(nodeCfgGen, ytsaurus, m, dndSpec))
		}
	}

	var s components.Component

	allComponents := []components.Component{
		d, m, yc,
	}
	allComponents = append(allComponents, dnds...)
	allComponents = append(allComponents, hps...)

	if resource.Spec.UI != nil {
		ui := components.NewUI(cfgen, ytsaurus, m)
		allComponents = append(allComponents, ui)
	}

	if len(resource.Spec.RPCProxies) > 0 {
		var rps []components.Component
		for _, rpSpec := range ytsaurus.GetResource().Spec.RPCProxies {
			rps = append(rps, components.NewRPCProxy(cfgen, ytsaurus, yc, m, rpSpec))
		}
		allComponents = append(allComponents, rps...)
	}

	if len(resource.Spec.TCPProxies) > 0 {
		var tps []components.Component
		for _, tpSpec := range ytsaurus.GetResource().Spec.TCPProxies {
			tps = append(tps, components.NewTCPProxy(cfgen, ytsaurus, m, tpSpec))
		}
		allComponents = append(allComponents, tps...)
	}

	if len(resource.Spec.KafkaProxies) > 0 {
		var kps []components.Component
		for _, kpSpec := range ytsaurus.GetResource().Spec.KafkaProxies {
			kps = append(kps, components.NewKafkaProxy(cfgen, ytsaurus, m, kpSpec))
		}
		allComponents = append(allComponents, kps...)
	}

	var ends []components.Component
	if len(resource.Spec.ExecNodes) > 0 {
		for _, endSpec := range ytsaurus.GetResource().Spec.ExecNodes {
			ends = append(ends, components.NewExecNode(nodeCfgGen, ytsaurus, m, endSpec))
		}
	}
	allComponents = append(allComponents, ends...)

	var tnds []components.Component
	if len(resource.Spec.TabletNodes) > 0 {
		for idx, tndSpec := range ytsaurus.GetResource().Spec.TabletNodes {
			tnds = append(tnds, components.NewTabletNode(nodeCfgGen, ytsaurus, yc, tndSpec, idx == 0))
		}
	}
	allComponents = append(allComponents, tnds...)

	if resource.Spec.Schedulers != nil {
		s = components.NewScheduler(cfgen, ytsaurus, m, ends, tnds)
		allComponents = append(allComponents, s)
	}

	if resource.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, ytsaurus, m)
		allComponents = append(allComponents, ca)
	}

	var q components.Component
	if resource.Spec.QueryTrackers != nil && resource.Spec.Schedulers != nil && len(resource.Spec.TabletNodes) > 0 {
		q = components.NewQueryTracker(cfgen, ytsaurus, yc, tnds)
		allComponents = append(allComponents, q)
	}

	if resource.Spec.QueueAgents != nil && len(resource.Spec.TabletNodes) > 0 {
		qa := components.NewQueueAgent(cfgen, ytsaurus, yc, m, tnds)
		allComponents = append(allComponents, qa)
	}

	var yqla components.Component
	if resource.Spec.YQLAgents != nil {
		yqla = components.NewYQLAgent(cfgen, ytsaurus, m)
		allComponents = append(allComponents, yqla)
	}

	if resource.Spec.StrawberryController != nil && resource.Spec.Schedulers != nil {
		strawberry := components.NewStrawberryController(cfgen, ytsaurus, m, s, dnds)
		allComponents = append(allComponents, strawberry)
	}

	if resource.Spec.MasterCaches != nil {
		mc := components.NewMasterCache(cfgen, ytsaurus)
		allComponents = append(allComponents, mc)
	}
	// Fetch component status.
	var readyComponents []string
	var updatingComponents []string
	var notReadyComponents []string

	status := ComponentManagerStatus{
		needInit:           false,
		needSync:           false,
		needUpdate:         nil,
		allReadyOrUpdating: true,
	}
	for _, c := range allComponents {
		err := c.Fetch(ctx)
		if err != nil {
			logger.Error(err, "failed to fetch status for controller", "component", c.GetFullName())
			return nil, err
		}

		componentStatus, err := c.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get component %s status: %w", c.GetFullName(), err)
		}

		c.SetReadyCondition(componentStatus)
		syncStatus := componentStatus.SyncStatus

		if syncStatus == components.SyncStatusNeedLocalUpdate {
			status.needUpdate = append(status.needUpdate, c)
		}

		if !components.IsRunningStatus(syncStatus) {
			if ytsaurus.GetClusterState() == ytv1.ClusterStateRunning {
				msg := fmt.Sprintf("component `%s` status is neither Ready nor NeedLocalUpdate, but `%s`, "+
					"needInit=true will be set, which will lead to Reconfiguration since cluster is in Running state",
					c.GetFullName(), syncStatus,
				)
				logger.Info(msg,
					"component", c.GetFullName(),
					"syncStatus", syncStatus,
				)
			}
			status.needInit = true
		}

		if syncStatus != components.SyncStatusReady && syncStatus != components.SyncStatusUpdating {
			status.allReadyOrUpdating = false
		}

		if syncStatus != components.SyncStatusReady {
			logger.Info("component is not ready", "component", c.GetFullName(), "syncStatus", syncStatus)
			notReadyComponents = append(notReadyComponents, c.GetFullName())
			status.needSync = true
		} else {
			readyComponents = append(readyComponents, c.GetFullName())
		}

		if syncStatus == components.SyncStatusUpdating {
			updatingComponents = append(updatingComponents, c.GetFullName())
		}
	}

	logger.Info("Ytsaurus sync status",
		"notReadyComponents", notReadyComponents,
		"readyComponents", readyComponents,
		"updatingComponents", updatingComponents,
		"updateState", resource.Status.UpdateStatus.State,
		"clusterState", resource.Status.State)

	return &ComponentManager{
		ytsaurus:      ytsaurus,
		allComponents: allComponents,
		status:        status,
	}, nil
}

func (cm *ComponentManager) Sync(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	hasPending := false
	for _, c := range cm.allComponents {
		status, err := c.Status(ctx)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("failed to get status for %s: %w", c.GetFullName(), err)
		}

		if status.SyncStatus == components.SyncStatusPending ||
			status.SyncStatus == components.SyncStatusUpdating {
			hasPending = true
			logger.Info("component sync", "component", c.GetFullName())
			if err := c.Sync(ctx); err != nil {
				logger.Error(err, "component sync failed", "component", c.GetFullName())
				return ctrl.Result{Requeue: true}, err
			}
		}
	}

	if err := cm.ytsaurus.APIProxy().UpdateStatus(ctx); err != nil {
		logger.Error(err, "update Ytsaurus status failed")
		return ctrl.Result{Requeue: true}, err
	}

	if !hasPending {
		// All components are blocked.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func (cm *ComponentManager) needSync() bool {
	return cm.status.needSync
}

func (cm *ComponentManager) needInit() bool {
	return cm.status.needInit
}

func (cm *ComponentManager) needUpdate() []components.Component {
	return cm.status.needUpdate
}

func (cm *ComponentManager) allReadyOrUpdating() bool {
	return cm.status.allReadyOrUpdating
}

func (cm *ComponentManager) arePodsRemoved() bool {
	for _, cmp := range cm.allComponents {
		if components.IsUpdatingComponent(cm.ytsaurus, cmp) && !cm.areComponentPodsRemoved(cmp) {
			return false
		}
	}

	return true
}

func (cm *ComponentManager) allUpdatableComponents() []components.Component {
	var result []components.Component
	for _, cmp := range cm.allComponents {
		if cmp.GetType() != consts.YtsaurusClientType {
			result = append(result, cmp)
		}
	}
	return result
}

func (cm *ComponentManager) areComponentPodsRemoved(component components.Component) bool {
	return cm.ytsaurus.IsUpdateStatusConditionTrue(component.GetLabeller().GetPodsRemovedCondition())
}
