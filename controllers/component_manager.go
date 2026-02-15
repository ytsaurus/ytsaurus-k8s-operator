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
	allReady           bool // All components are Ready - no reconciliations required
	allRunning         bool // All components are Ready or NeedUpdate - can start updates
	allReadyOrUpdating bool // All components are Ready or Updating - no new updates

	needUpdate   []ytv1.Component // Components in state NeedUpdate
	canUpdate    []ytv1.Component // Components with update allowed
	cannotUpdate []ytv1.Component // Components with update blocked
	nowUpdating  []ytv1.Component // Components updating right now
}

func NewComponentManager(
	ctx context.Context,
	ytsaurus *apiProxy.Ytsaurus,
	clusterDomain string,
) (*ComponentManager, error) {
	resource := ytsaurus.GetResource()

	if clusterDomain == "" {
		return nil, fmt.Errorf("cluster domain is not defined")
	}

	cfgen := ytconfig.NewGenerator(resource, clusterDomain)
	nodeCfgGen := &cfgen.NodeGenerator

	var allComponents []components.Component

	getAllComponents := func() []components.Component {
		return allComponents
	}

	m := components.NewMaster(cfgen, ytsaurus)
	var hps []components.Component
	for _, hpSpec := range ytsaurus.GetResource().Spec.HTTPProxies {
		hps = append(hps, components.NewHTTPProxy(cfgen, ytsaurus, m, hpSpec))
	}
	yc := components.NewYtsaurusClient(cfgen, ytsaurus, hps[0], getAllComponents)
	d := components.NewDiscovery(cfgen, ytsaurus, yc)
	ih := components.NewImageHeater(cfgen, ytsaurus, getAllComponents)

	var dnds []components.Component
	if len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range ytsaurus.GetResource().Spec.DataNodes {
			dnds = append(dnds, components.NewDataNode(nodeCfgGen, ytsaurus, m, dndSpec))
		}
	}

	allComponents = append(allComponents, d, m, yc, ih)
	allComponents = append(allComponents, dnds...)
	allComponents = append(allComponents, hps...)

	if resource.Spec.UI != nil {
		ui := components.NewUI(cfgen, ytsaurus, m)
		allComponents = append(allComponents, ui)
	}

	if len(resource.Spec.RPCProxies) > 0 {
		var rps []components.Component
		for _, rpSpec := range ytsaurus.GetResource().Spec.RPCProxies {
			rps = append(rps, components.NewRPCProxy(cfgen, ytsaurus, m, rpSpec))
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

	var sch components.Component
	if resource.Spec.Schedulers != nil {
		sch = components.NewScheduler(cfgen, ytsaurus, m, yc, ends, tnds)
		allComponents = append(allComponents, sch)
	}

	if resource.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, ytsaurus, m, yc)
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
		yqla = components.NewYQLAgent(cfgen, ytsaurus, yc, m)
		allComponents = append(allComponents, yqla)
	}

	if resource.Spec.StrawberryController != nil && resource.Spec.Schedulers != nil {
		strawberry := components.NewStrawberryController(cfgen, ytsaurus, m, sch, dnds)
		allComponents = append(allComponents, strawberry)
	}

	if resource.Spec.MasterCaches != nil {
		mc := components.NewMasterCache(cfgen, ytsaurus, yc)
		allComponents = append(allComponents, mc)
	}

	if resource.Spec.CypressProxies != nil {
		cyp := components.NewCypressProxy(cfgen, ytsaurus)
		allComponents = append(allComponents, cyp)
	}

	if resource.Spec.BundleController != nil {
		bc := components.NewBundleController(cfgen, ytsaurus)
		allComponents = append(allComponents, bc)
	}

	if resource.Spec.TabletBalancer != nil {
		tb := components.NewTabletBalancer(cfgen, ytsaurus)
		allComponents = append(allComponents, tb)
	}

	tt := components.NewTimbertruck(cfgen, ytsaurus, tnds, yc)
	allComponents = append(allComponents, tt)

	return &ComponentManager{
		ytsaurus:      ytsaurus,
		allComponents: allComponents,
	}, nil
}

func (cm *ComponentManager) FetchStatus(ctx context.Context) error {
	logger := log.FromContext(ctx)
	resource := cm.ytsaurus.GetResource()

	// Fetch component status.
	var readyComponents []string
	var needUpdateComponents []string
	var updatingComponents []string
	var notReadyComponents []string

	cm.status = ComponentManagerStatus{
		allReady:           true,
		allRunning:         true,
		allReadyOrUpdating: true,
		needUpdate:         nil,
	}

	for _, component := range cm.allComponents {
		err := component.Fetch(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch component %s: %w", component.GetFullName(), err)
		}

		status, err := component.Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to get component %s status: %w", component.GetFullName(), err)
		}

		component.SetReadyCondition(status)
		logger.Info("Component status",
			"component", component.GetFullName(),
			"status", status.SyncStatus,
			"message", status.Message,
		)

		switch status.SyncStatus {
		case components.SyncStatusReady:
			readyComponents = append(readyComponents, component.GetFullName())
		case components.SyncStatusNeedUpdate:
			needUpdateComponents = append(needUpdateComponents, component.GetFullName())
			cm.status.needUpdate = append(cm.status.needUpdate, component.GetComponent())
			cm.status.allReady = false
			cm.status.allReadyOrUpdating = false
		case components.SyncStatusUpdating:
			updatingComponents = append(updatingComponents, component.GetFullName())
			cm.status.allReady = false
			cm.status.allRunning = false
		default:
			notReadyComponents = append(notReadyComponents, component.GetFullName())
			cm.status.allReady = false
			cm.status.allRunning = false
			cm.status.allReadyOrUpdating = false
			if cm.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning {
				logger.Info("Cluster needs reconfiguration because component is not running",
					"component", component.GetFullName(),
					"status", status.SyncStatus,
					"message", status.Message,
				)
			}
		}
	}

	logger.Info("Ytsaurus sync status",
		"clusterState", resource.Status.State,
		"updateState", resource.Status.UpdateStatus.State,
		"allReady", cm.status.allReady,
		"allRunning", cm.status.allRunning,
		"allReadyOrUpdating", cm.status.allReadyOrUpdating,
		"needUpdateComponents", needUpdateComponents,
		"updatingComponents", updatingComponents,
		"notReadyComponents", notReadyComponents,
		"readyComponents", readyComponents,
	)

	return nil
}

func (cm *ComponentManager) Sync(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	handled, res, err := cm.runImageHeaterIfEnabled(ctx)
	if err != nil {
		return res, err
	}
	if handled {
		return res, nil
	}

	hasPending := false
	var syncErr error
	for _, c := range cm.allComponents {
		status, err := c.Status(ctx)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("failed to get status for %s: %w", c.GetFullName(), err)
		}

		if status.SyncStatus == components.SyncStatusPending ||
			status.SyncStatus == components.SyncStatusUpdating {
			hasPending = true
			logger.Info("Sync component",
				"component", c.GetFullName(),
				"status", status.SyncStatus,
				"message", status.Message,
			)
			if err := c.Sync(ctx); err != nil {
				logger.Error(err, "Cannot sync component",
					"component", c.GetFullName(),
				)
				syncErr = err
				break
			}
		}
	}

	if err := cm.ytsaurus.UpdateStatus(ctx); err != nil {
		logger.Error(err, "Cannot update ytsaurus status")
		return ctrl.Result{Requeue: true}, err
	}

	if syncErr != nil {
		return ctrl.Result{Requeue: true}, syncErr
	}

	if !hasPending {
		// All components are blocked.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
}

func (cm *ComponentManager) arePodsRemoved() bool {
	for _, cmp := range cm.allComponents {
		if cmp.GetType() == consts.YtsaurusClientType || cmp.GetType() == consts.ImageHeaterType {
			continue
		}
		if components.IsUpdatingComponent(cm.ytsaurus, cmp) && !cm.areComponentPodsRemoved(cmp) {
			return false
		}
	}

	return true
}

func (cm *ComponentManager) allUpdatableComponents() []ytv1.Component {
	var result []ytv1.Component
	for _, cmp := range cm.allComponents {
		if cmp.GetType() != consts.YtsaurusClientType && cmp.GetType() != consts.TimbertruckType {
			result = append(result, cmp.GetComponent())
		}
	}
	return result
}

func (cm *ComponentManager) applyUpdatePlan(updatePlan []ytv1.ComponentUpdateSelector) {
	// TODO: Inline code.
	cm.status.canUpdate, cm.status.cannotUpdate = chooseUpdatingComponents(
		updatePlan,
		cm.status.needUpdate,
		cm.allUpdatableComponents(),
	)
}

func (cm *ComponentManager) areComponentPodsRemoved(component components.Component) bool {
	// Check for either PodsRemoved (bulk update) or PodsUpdated (OnDelete mode)
	return cm.ytsaurus.IsUpdateStatusConditionTrue(component.GetLabeller().GetPodsRemovedCondition()) ||
		cm.ytsaurus.IsUpdateStatusConditionTrue(component.GetLabeller().GetPodsUpdatedCondition())
}

func (cm *ComponentManager) runImageHeaterIfEnabled(ctx context.Context) (handled bool, result ctrl.Result, err error) {
	logger := log.FromContext(ctx)
	state := cm.ytsaurus.GetClusterState()
	if (state != ytv1.ClusterStateInitializing &&
		state != ytv1.ClusterStateCreated &&
		state != ytv1.ClusterState("")) || !cm.ytsaurus.IsImageHeaterEnabled() {
		return false, ctrl.Result{}, nil
	}

	ih := cm.findImageHeater()
	if ih == nil {
		return false, ctrl.Result{}, nil
	}

	status, err := ih.Status(ctx)
	if err != nil {
		return true, ctrl.Result{Requeue: true}, fmt.Errorf("failed to get status for %s: %w", ih.GetFullName(), err)
	}
	if status.SyncStatus == components.SyncStatusReady {
		return false, ctrl.Result{}, nil
	}

	if status.SyncStatus == components.SyncStatusPending ||
		status.SyncStatus == components.SyncStatusUpdating ||
		status.SyncStatus == components.SyncStatusNeedUpdate {
		logger.Info("component sync", "component", ih.GetFullName())
		if err := ih.Sync(ctx); err != nil {
			logger.Error(err, "component sync failed", "component", ih.GetFullName())
			return true, ctrl.Result{Requeue: true}, err
		}
	}

	if err := cm.ytsaurus.UpdateStatus(ctx); err != nil {
		logger.Error(err, "update Ytsaurus status failed")
		return true, ctrl.Result{Requeue: true}, err
	}

	if status.SyncStatus == components.SyncStatusBlocked {
		return true, ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return true, ctrl.Result{RequeueAfter: time.Second}, nil
}

func (cm *ComponentManager) findImageHeater() components.Component {
	for _, cmp := range cm.allComponents {
		if cmp.GetType() == consts.ImageHeaterType {
			return cmp
		}
	}
	return nil
}

func canUpdateComponent(selectors []ytv1.ComponentUpdateSelector, component ytv1.Component) bool {
	for _, selector := range selectors {
		if selector.Class != consts.ComponentClassUnspecified {
			switch selector.Class {
			case consts.ComponentClassEverything:
				return true
			case consts.ComponentClassNothing:
				return false
			case consts.ComponentClassStateless:
				if component.Type != consts.DataNodeType && component.Type != consts.TabletNodeType && component.Type != consts.MasterType {
					return true
				}
			default:
				return false
			}
		}
		if selector.Component.Type == component.Type && (selector.Component.Name == "" || selector.Component.Name == component.Name) {
			return true
		}
	}
	return false
}

// Considers splits all the components in two groups: ones that can be updated and ones which update is blocked.
func chooseUpdatingComponents(selectors []ytv1.ComponentUpdateSelector, needUpdate []ytv1.Component, allComponents []ytv1.Component) (canUpdate []ytv1.Component, cannotUpdate []ytv1.Component) {
	for _, component := range needUpdate {
		upd := canUpdateComponent(selectors, component)
		if upd {
			canUpdate = append(canUpdate, component)
		} else {
			cannotUpdate = append(cannotUpdate, component)
		}
	}

	if len(canUpdate) == 0 {
		return nil, cannotUpdate
	}
	if hasEverythingSelector(selectors) && needFullUpdate(needUpdate) {
		// if image wasn't changed, we don't need to run ImageHeater
		if !hasComponent(needUpdate, consts.ImageHeaterType) {
			filtered := make([]ytv1.Component, 0, len(allComponents))
			for _, component := range allComponents {
				if component.Type == consts.ImageHeaterType {
					continue
				}
				filtered = append(filtered, component)
			}
			return filtered, nil
		}
		// Here we update not only components that are not up-to-date, but all cluster.
		// FIXME: Why?
		return allComponents, nil
	}
	return canUpdate, cannotUpdate
}

func hasEverythingSelector(selectors []ytv1.ComponentUpdateSelector) bool {
	for _, selector := range selectors {
		if selector.Class == consts.ComponentClassEverything {
			return true
		}
	}

	return false
}

func needFullUpdate(needUpdate []ytv1.Component) bool {
	statelessSelector := []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassStateless}}
	for _, component := range needUpdate {
		isStateless := canUpdateComponent(statelessSelector, component)
		if !isStateless {
			return true
		}
	}
	return false
}
