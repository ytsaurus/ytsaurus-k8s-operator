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
	apiProxy        *apiProxy.APIProxy
	allComponents   []components.Component
	masterComponent components.ServerComponent
	status          ComponentStatus
}

func NewComponentManager(
	ctx context.Context,
	apiProxy *apiProxy.APIProxy,
) (*ComponentManager, error) {
	logger := log.FromContext(ctx)
	ytsaurus := apiProxy.Ytsaurus()

	cfgen := ytconfig.NewGenerator(ytsaurus, getClusterDomain(apiProxy.Client()))

	d := components.NewDiscovery(cfgen, apiProxy)
	m := components.NewMaster(cfgen, apiProxy)
	var hps []components.Component
	for _, hpSpec := range apiProxy.Ytsaurus().Spec.HTTPProxies {
		hps = append(hps, components.NewHTTPProxy(cfgen, apiProxy, m, hpSpec))
	}
	yc := components.NewYtsaurusClient(cfgen, apiProxy, hps[0])

	dn := components.NewDataNode(cfgen, apiProxy, m)

	var en, tn, s components.Component

	allComponents := []components.Component{
		d, m, dn, yc,
	}
	allComponents = append(allComponents, hps...)

	if ytsaurus.Spec.UI != nil {
		ui := components.NewUI(cfgen, apiProxy, m)
		allComponents = append(allComponents, ui)
	}

	if ytsaurus.Spec.RPCProxies != nil && len(ytsaurus.Spec.RPCProxies) > 0 {
		var rps []components.Component
		for _, rpSpec := range apiProxy.Ytsaurus().Spec.RPCProxies {
			rps = append(rps, components.NewRPCProxy(cfgen, apiProxy, m, rpSpec))
		}
		allComponents = append(allComponents, rps...)
	}

	if ytsaurus.Spec.ExecNodes != nil && len(ytsaurus.Spec.ExecNodes) > 0 {
		en = components.NewExecNode(cfgen, apiProxy, m)
		allComponents = append(allComponents, en)
	}

	if ytsaurus.Spec.TabletNodes != nil && len(ytsaurus.Spec.TabletNodes) > 0 {
		tn = components.NewTabletNode(cfgen, apiProxy, yc)
		allComponents = append(allComponents, tn)
	}

	if ytsaurus.Spec.Schedulers != nil {
		s = components.NewScheduler(cfgen, apiProxy, m, en, tn)
		allComponents = append(allComponents, s)
	}

	if ytsaurus.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, apiProxy, m)
		allComponents = append(allComponents, ca)
	}

	if ytsaurus.Spec.QueryTrackers != nil {
		q := components.NewQueryTracker(cfgen, apiProxy, m)
		allComponents = append(allComponents, q)
	}

	if ytsaurus.Spec.YQLAgents != nil {
		yqla := components.NewYQLAgent(cfgen, apiProxy, m)
		allComponents = append(allComponents, yqla)
	}

	if ytsaurus.Spec.Chyt != nil {
		chyt := components.NewChytController(cfgen, apiProxy, m, dn)
		allComponents = append(allComponents, chyt)
	}

	if ytsaurus.Spec.Spyt != nil && en != nil {
		spyt := components.NewSpyt(cfgen, apiProxy, m, en, s, dn)
		allComponents = append(allComponents, spyt)
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
		"updateState", ytsaurus.Status.UpdateStatus.State,
		"clusterState", ytsaurus.Status.State)

	return &ComponentManager{
		apiProxy:        apiProxy,
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

func (cm *ComponentManager) areServerPodsRemoved(proxy *apiProxy.APIProxy) bool {
	for _, cmp := range cm.allComponents {
		if scmp, ok := cmp.(components.ServerComponent); ok {
			if !proxy.IsUpdateStatusConditionTrue(scmp.GetPodsRemovedCondition()) {
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
