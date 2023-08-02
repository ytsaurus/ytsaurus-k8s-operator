package controllers

import (
	"context"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

type ComponentManager struct {
	ytsaurus        *apiProxy.Ytsaurus
	allComponents   []components.Component
	masterComponent components.ServerComponent
	status          ComponentStatus
}

func NewComponentManager(
	ctx context.Context,
	ytsaurus *apiProxy.Ytsaurus,
) (*ComponentManager, error) {
	logger := log.FromContext(ctx)
	resource := ytsaurus.GetResource()

	cfgen := ytconfig.NewGenerator(resource, getClusterDomain(ytsaurus.APIProxy().Client()))

	d := components.NewDiscovery(cfgen, ytsaurus)
	m := components.NewMaster(cfgen, ytsaurus)
	var hps []components.Component
	for _, hpSpec := range ytsaurus.GetResource().Spec.HTTPProxies {
		hps = append(hps, components.NewHTTPProxy(cfgen, ytsaurus, m, hpSpec))
	}
	yc := components.NewYtsaurusClient(cfgen, ytsaurus, hps[0])

	var dnds []components.Component
	if resource.Spec.DataNodes != nil && len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range ytsaurus.GetResource().Spec.DataNodes {
			dnds = append(dnds, components.NewDataNode(cfgen, ytsaurus, m, dndSpec))
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

	if resource.Spec.RPCProxies != nil && len(resource.Spec.RPCProxies) > 0 {
		var rps []components.Component
		for _, rpSpec := range ytsaurus.GetResource().Spec.RPCProxies {
			rps = append(rps, components.NewRPCProxy(cfgen, ytsaurus, m, rpSpec))
		}
		allComponents = append(allComponents, rps...)
	}

	var ends []components.Component
	if resource.Spec.ExecNodes != nil && len(resource.Spec.ExecNodes) > 0 {
		for _, endSpec := range ytsaurus.GetResource().Spec.ExecNodes {
			ends = append(ends, components.NewExecNode(cfgen, ytsaurus, m, endSpec))
		}
	}
	allComponents = append(allComponents, ends...)

	var tnds []components.Component
	if resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		for idx, tndSpec := range ytsaurus.GetResource().Spec.TabletNodes {
			tnds = append(tnds, components.NewTabletNode(cfgen, ytsaurus, yc, tndSpec, idx == 0))
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

	if resource.Spec.QueryTrackers != nil {
		q := components.NewQueryTracker(cfgen, ytsaurus, yc)
		allComponents = append(allComponents, q)
	}

	if resource.Spec.YQLAgents != nil {
		yqla := components.NewYQLAgent(cfgen, ytsaurus, m)
		allComponents = append(allComponents, yqla)
	}

	if resource.Spec.Chyt != nil && resource.Spec.Schedulers != nil {
		chyt := components.NewChytController(cfgen, ytsaurus, m, s, dnds)
		allComponents = append(allComponents, chyt)
	}

	// Fetch component status.
	var readyComponents []string
	var notReadyComponents []string

	componentStatus := ComponentStatus{needSync: false, needUpdate: false, allReadyOrUpdating: true}
	for _, c := range allComponents {
		err := c.Fetch(ctx)
		if err != nil {
			logger.Error(err, "failed to fetch status for controller", "component", c.GetName())
			return nil, err
		}

		status := c.Status(ctx)

		if status == components.SyncStatusNeedUpdate {
			componentStatus.needUpdate = true
		}

		if status != components.SyncStatusReady && status != components.SyncStatusUpdating {
			componentStatus.allReadyOrUpdating = false
		}

		if status != components.SyncStatusReady {
			logger.Info("component is not ready", "component", c.GetName(), "syncStatus", status)
			notReadyComponents = append(notReadyComponents, c.GetName())
			componentStatus.needSync = true
		} else {
			readyComponents = append(readyComponents, c.GetName())
		}
	}

	logger.Info("Ytsaurus sync status",
		"notReadyComponents", notReadyComponents,
		"readyComponents", readyComponents,
		"updateState", resource.Status.UpdateStatus.State,
		"clusterState", resource.Status.State)

	return &ComponentManager{
		ytsaurus:        ytsaurus,
		allComponents:   allComponents,
		masterComponent: m,
		status:          componentStatus,
	}, nil
}

func (cm *ComponentManager) needSync() bool {
	return cm.status.needSync
}

func (cm *ComponentManager) needUpdate() bool {
	return cm.status.needUpdate
}

func (cm *ComponentManager) allReadyOrUpdating() bool {
	return cm.status.allReadyOrUpdating
}

func (cm *ComponentManager) areServerPodsRemoved(ytsaurus *apiProxy.Ytsaurus) bool {
	for _, cmp := range cm.allComponents {
		if scmp, ok := cmp.(components.ServerComponent); ok {
			if !ytsaurus.IsUpdateStatusConditionTrue(scmp.GetPodsRemovedCondition()) {
				return false
			}
		}
	}
	return true
}

func (cm *ComponentManager) Sync(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	hasPending := false
	for _, c := range cm.allComponents {
		status := c.Status(ctx)

		if status == components.SyncStatusPending || status == components.SyncStatusUpdating {
			hasPending = true
			logger.Info("component sync", "component", c.GetName())
			if err := c.Sync(ctx); err != nil {
				logger.Error(err, "component sync failed", "component", c.GetName())
				return ctrl.Result{Requeue: true}, err
			}
		}
	}

	if !hasPending {
		// All components are blocked.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
}
