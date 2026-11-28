package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	getHeaterStatus func(ytv1.Component) components.ComponentStatus
	// An unreadable group cannot act on partial observations. Other groups can
	// still recover their workloads during the same reconcile.
	groupErrors   map[ytv1.Component]error
	deferred      map[ytv1.Component]bool
	targetChanged bool
}

type ComponentManagerStatus struct {
	masterReady bool // All masters are Ready - quorum may enter/exit read-only state
	allStarted  bool // All components are Started or Running - no reconciliations required
	allRunning  bool // All components are Ready or NeedUpdate - can start updates
	allReady    bool // All components are Ready - no reconciliations required

	pending []ytv1.Component // Components in state Pending or Updating
	blocked []ytv1.Component // Components in state Blocked
	started []ytv1.Component // Components in state Started

	needUpdate   []ytv1.Component               // Components in state NeedUpdate
	canUpdate    []ytv1.Component               // Components with update allowed
	cannotUpdate []ytv1.Component               // Components with update blocked
	admitted     []ytv1.ComponentUpdateProgress // Exact selector policy chosen for each admitted group
	nowUpdating  []ytv1.Component               // Components updating right now

	clusterMaintenance bool
	mastersMaintenance bool
	shutdownStorage    bool
	shutdownTablets    bool
	shutdownCompute    bool

	removeTabletCellsOnUpdate bool // Skip tablet cell save/remove/recover when tablet nodes use OnDelete strategy
	masterHotUpdate           bool // Master update uses OnDelete — skip safety steps (safe mode, snapshots, exit read-only)
}

//nolint:cyclop //shush
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

	// NOTE: Must be first component for blocking cluster initialization.
	var getHeaterStatus func(ytv1.Component) components.ComponentStatus
	if ytsaurus.GetImageHeater("") != nil {
		ih := components.NewImageHeater(cfgen, ytsaurus, getAllComponents)
		allComponents = append(allComponents, ih)
		getHeaterStatus = ih.GetHeaterStatus
	}

	var secondaryMasters []*components.Master
	for i := range resource.Spec.SecondaryMasters {
		sm := components.NewMaster(cfgen, ytsaurus, &resource.Spec.SecondaryMasters[i], nil)
		secondaryMasters = append(secondaryMasters, sm)
		allComponents = append(allComponents, sm)
	}

	// NOTE: Primary master readiness depends on readiness of secondary masters.
	m := components.NewMaster(cfgen, ytsaurus, &resource.Spec.PrimaryMasters, secondaryMasters)
	allComponents = append(allComponents, m)

	var defaultHTTPProxy *components.HttpProxy
	for _, hpSpec := range resource.Spec.HTTPProxies {
		hp := components.NewHTTPProxy(cfgen, ytsaurus, m, hpSpec)
		allComponents = append(allComponents, hp)
		if hp.GetShortName() == "" {
			defaultHTTPProxy = hp
		}
	}
	if defaultHTTPProxy == nil {
		return nil, fmt.Errorf("HTTP proxy with role %q is required", consts.DefaultHTTPProxyRole)
	}

	yc := components.NewYtsaurusClient(cfgen, ytsaurus, m, defaultHTTPProxy, getAllComponents)
	allComponents = append(allComponents, yc)

	d := components.NewDiscovery(cfgen, ytsaurus, yc, m)
	allComponents = append(allComponents, d)

	var dnds []components.Component
	if len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range resource.Spec.DataNodes {
			dnds = append(dnds, components.NewDataNode(nodeCfgGen, ytsaurus, m, yc, dndSpec))
		}
	}
	allComponents = append(allComponents, dnds...)

	if resource.Spec.UI != nil {
		ui := components.NewUI(cfgen, ytsaurus, m)
		allComponents = append(allComponents, ui)
	}

	if len(resource.Spec.RPCProxies) > 0 {
		var rps []components.Component
		for _, rpSpec := range resource.Spec.RPCProxies {
			rps = append(rps, components.NewRPCProxy(cfgen, ytsaurus, m, rpSpec))
		}
		allComponents = append(allComponents, rps...)
	}

	if len(resource.Spec.TCPProxies) > 0 {
		var tps []components.Component
		for _, tpSpec := range resource.Spec.TCPProxies {
			tps = append(tps, components.NewTCPProxy(cfgen, ytsaurus, m, tpSpec))
		}
		allComponents = append(allComponents, tps...)
	}

	if len(resource.Spec.KafkaProxies) > 0 {
		var kps []components.Component
		for _, kpSpec := range resource.Spec.KafkaProxies {
			kps = append(kps, components.NewKafkaProxy(cfgen, ytsaurus, m, kpSpec))
		}
		allComponents = append(allComponents, kps...)
	}

	for _, endSpec := range resource.Spec.ExecNodes {
		allComponents = append(allComponents, components.NewExecNode(nodeCfgGen, ytsaurus, m, endSpec, yc))
	}

	var tnds []components.Component
	if len(resource.Spec.TabletNodes) > 0 {
		for idx, tndSpec := range resource.Spec.TabletNodes {
			tnds = append(tnds, components.NewTabletNode(nodeCfgGen, ytsaurus, yc, m, tndSpec, idx == 0))
		}
	}
	allComponents = append(allComponents, tnds...)

	var sch components.Component
	if resource.Spec.Schedulers != nil {
		sch = components.NewScheduler(cfgen, ytsaurus, m, yc, tnds)
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
		mc := components.NewMasterCache(cfgen, ytsaurus, yc, m)
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

	if components.TimbertruckDeliveryEnabled(&resource.Spec) {
		tt := components.NewTimbertruck(cfgen, ytsaurus, tnds, yc)
		allComponents = append(allComponents, tt)
	}

	return &ComponentManager{
		ytsaurus:        ytsaurus,
		allComponents:   allComponents,
		getHeaterStatus: getHeaterStatus,
	}, nil
}

func (cm *ComponentManager) FetchStatus(ctx context.Context) error {
	logger := log.FromContext(ctx)
	resource := cm.ytsaurus.GetResource()
	maintenance := cm.ytsaurus.GetClusterMaintenance()
	shutdown := maintenance.Shutdown

	// Fetch component status.
	var readyComponents []ytv1.Component

	cm.status = ComponentManagerStatus{
		masterReady: true,
		allStarted:  true,
		allRunning:  true,
		allReady:    true,

		clusterMaintenance: shutdown != ytv1.ClusterShutdownNone,
		mastersMaintenance: shutdown == ytv1.ClusterShutdownExceptMasters,
		shutdownCompute:    shutdown == ytv1.ClusterShutdownExceptMasters || shutdown == ytv1.ClusterShutdownEverything || shutdown == ytv1.ClusterShutdownCompute,
		shutdownStorage:    shutdown == ytv1.ClusterShutdownExceptMasters || shutdown == ytv1.ClusterShutdownEverything,
		shutdownTablets:    shutdown == ytv1.ClusterShutdownExceptMasters || shutdown == ytv1.ClusterShutdownEverything || shutdown == ytv1.ClusterShutdownTablets,
	}

	for _, component := range cm.allComponents {
		var status components.ComponentStatus
		err := component.Fetch(ctx)
		if err == nil {
			err = cm.refreshGroupTarget(component)
		}
		if err == nil {
			if cm.ytsaurus.IsReadyToUpdate() {
				status = component.NeedUpdate()
			}
			if !status.IsNeedUpdate() {
				status, err = cm.observeComponent(ctx, component)
			}
		}
		if err != nil {
			err = fmt.Errorf("observing %s: %w", component.GetFullName(), err)
			if !cm.recordGroupError(component, err) {
				return err
			}
			status = components.ComponentStatusBlocked("%v", err)
		}

		component.SetStatus(status)
		component.SetReadyCondition(status)
		logger.Info("Component status",
			"component", component.GetFullName(),
			"status", status.SyncStatus,
			"message", status.Message,
		)

		if component.GetType() == consts.YtsaurusClientType && !status.IsReady() {
			// Virtual component without pods can be non-ready, but not non-running.
			// They should not initiate cluster reconfiguration.
			cm.status.pending = append(cm.status.pending, component.GetComponent())
			cm.status.allReady = false
			continue
		}

		if component.GetType() == consts.MasterType && !status.IsReady() {
			cm.status.masterReady = false
		}

		switch status.SyncStatus {
		case components.SyncStatusReady:
			readyComponents = append(readyComponents, component.GetComponent())
		case components.SyncStatusNeedUpdate:
			cm.status.needUpdate = append(cm.status.needUpdate, component.GetComponent())
			cm.status.allReady = false
		case components.SyncStatusStarted:
			cm.status.started = append(cm.status.started, component.GetComponent())
			cm.status.allReady = false
			cm.status.allRunning = false
		case components.SyncStatusPending, components.SyncStatusUpdating:
			cm.status.pending = append(cm.status.pending, component.GetComponent())
			cm.status.allReady = false
			cm.status.allRunning = false
			cm.status.allStarted = false
		case components.SyncStatusBlocked:
			cm.status.blocked = append(cm.status.blocked, component.GetComponent())
			cm.status.allReady = false
			cm.status.allRunning = false
			cm.status.allStarted = false
		default:
			return fmt.Errorf("component %v has unknown sync status: %v", component.GetFullName(), status.SyncStatus)
		}
	}

	logger.Info("Ytsaurus sync status",
		"clusterState", resource.Status.State,
		"updateState", resource.Status.UpdateStatus.State,
		"masterReady", cm.status.masterReady,
		"allStarted", cm.status.allStarted,
		"allRunning", cm.status.allRunning,
		"allReady", cm.status.allReady,
		"needUpdate", cm.status.needUpdate,
		"pending", cm.status.pending,
		"blocked", cm.status.blocked,
		"started", cm.status.started,
		"readyComponents", readyComponents,
	)

	return nil
}

func (cm *ComponentManager) Sync(ctx context.Context) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if cm.ytsaurus.GetImageHeater("") == nil {
		cm.ytsaurus.RemoveStatusCondition(consts.ConditionImageHeaterReady)
		cm.ytsaurus.RemoveStatusCondition(consts.ConditionImageHeaterComplete)
	}

	if cm.status.clusterMaintenance {
		shutdown := cm.ytsaurus.GetClusterMaintenance().Shutdown
		cm.ytsaurus.SetStatusCondition(metav1.Condition{
			Type:    consts.ConditionClusterMaintenance,
			Status:  metav1.ConditionTrue,
			Reason:  string(shutdown),
			Message: fmt.Sprintf("Cluster is under maintenance, shutdown %v", shutdown),
		})
	} else {
		cm.ytsaurus.RemoveStatusCondition(consts.ConditionClusterMaintenance)
	}

	hasPending := false
	hasStarted := false
	var syncErr error
	for _, c := range cm.allComponents {
		if err := cm.groupErrors[c.GetComponent()]; err != nil {
			syncErr = errors.Join(syncErr, err)
			continue
		}
		status, err := cm.observeComponent(ctx, c)
		if err != nil {
			err = fmt.Errorf("failed to get status for %s: %w", c.GetFullName(), err)
			if cm.recordGroupError(c, err) {
				syncErr = errors.Join(syncErr, err)
				continue
			}
			return ctrl.Result{Requeue: true}, err
		}
		c.SetStatus(status)

		if status.SyncStatus == components.SyncStatusPending || status.SyncStatus == components.SyncStatusUpdating {
			hasPending = true
			logger.Info("Sync component",
				"component", c.GetFullName(),
				"status", status.SyncStatus,
				"message", status.Message,
			)
			actionStatus, actionErr := c.Sync(ctx, false)
			if actionErr != nil {
				logger.Error(actionErr, "Cannot sync component", "component", c.GetFullName(),
					"status", actionStatus.SyncStatus, "message", actionStatus.Message)
				syncErr = errors.Join(syncErr, actionErr)
				if cm.recordGroupError(c, actionErr) {
					continue
				}
				break
			}
		} else if status.SyncStatus == components.SyncStatusStarted {
			hasStarted = true
		}

		if cm.ytsaurus.IsInitializing() && c.GetType() == consts.ImageHeaterType {
			complete := cm.ytsaurus.GetStatusCondition(consts.ConditionImageHeaterComplete)
			if complete != nil && complete.Status != metav1.ConditionTrue {
				logger.Info("Cluster initialization is waiting for image heater", "message", complete.Message)
				cm.ytsaurus.RecordNormal("Initialization", "Waiting for image heater completion: "+complete.Message)
				break
			}
		}
	}

	if err := cm.ytsaurus.UpdateStatus(ctx); err != nil {
		logger.Error(err, "Cannot update ytsaurus status")
		return ctrl.Result{Requeue: true}, err
	}

	switch {
	case syncErr != nil:
		return ctrl.Result{Requeue: true}, syncErr
	case hasPending:
		return ctrl.Result{RequeueAfter: time.Second}, nil
	case hasStarted:
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	default:
		return ctrl.Result{RequeueAfter: consts.DefaultClusterStatusPollPeriod}, nil
	}
}

func (cm *ComponentManager) observeComponent(ctx context.Context, c components.Component) (components.ComponentStatus, error) {
	if cm.deferred[c.GetComponent()] {
		if c.GetType() == consts.MasterType && cm.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForMasterReady {
			return c.ArePodsReady(ctx)
		}
		return components.ComponentStatusNeedUpdate("New workload target pending the next update"), nil
	}
	switch cm.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForMasterReady, ytv1.UpdateStateWaitingForMasterExitReadOnly:
		// In an Everything update, normal replica repair must wait until masters are writable.
		if c.GetType() != consts.MasterType && cm.ytsaurus.GetGroupProgress(c.GetComponent()) != nil {
			return components.ComponentStatusBlocked("Waiting for master recovery"), nil
		}
	}
	return c.Sync(ctx, true)
}

func (cm *ComponentManager) recordGroupError(c components.Component, err error) bool {
	state := cm.ytsaurus.GetUpdateState()
	p := cm.ytsaurus.GetGroupProgress(c.GetComponent())
	if p == nil || (state != ytv1.UpdateStateWaitingForPodsRemoval && state != ytv1.UpdateStateWaitingForPodsCreation) {
		return false
	}
	if cm.groupErrors == nil {
		cm.groupErrors = make(map[ytv1.Component]error)
	}
	cm.groupErrors[c.GetComponent()] = err
	p.Reason = err.Error()
	c.SetStatus(components.ComponentStatusBlocked("%v", err))
	return true
}

func (cm *ComponentManager) allUpdatingImagesAreHeated() bool {
	if cm.getHeaterStatus != nil {
		for _, component := range cm.status.nowUpdating {
			if !cm.getHeaterStatus(component).IsReady() {
				return false
			}
		}
	}
	return true
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

func (cm *ComponentManager) areComponentPodsRemoved(component components.Component) bool {
	// Check for either PodsRemoved (bulk update) or PodsUpdated (OnDelete mode)
	return cm.ytsaurus.IsUpdateStatusConditionTrue(component.GetLabeller().GetPodsRemovedCondition()) ||
		cm.ytsaurus.IsUpdateStatusConditionTrue(component.GetLabeller().GetPodsUpdatedCondition())
}

func (cm *ComponentManager) applyUpdatePlan(updatePlan []ytv1.ComponentUpdateSelector) {
	cm.status.canUpdate = nil
	cm.status.admitted = nil
	cm.status.cannotUpdate = nil
	count := make([]int32, len(updatePlan))
	for _, component := range cm.status.needUpdate {
		index := -1
		for i, selector := range updatePlan {
			// An explicit master plan selects one cell. A stored mixed plan may predate webhook validation.
			if selector.Component.Type == consts.MasterType && !cm.status.mastersMaintenance &&
				(len(updatePlan) != 1 || selector.Component != component) {
				continue
			}
			if !canUpdateComponent(selector, component) {
				continue
			}
			if selector.Concurrency != nil && *selector.Concurrency <= count[i] {
				continue
			}
			index = i
			break
		}
		if index >= 0 {
			count[index] += 1
			cm.status.canUpdate = append(cm.status.canUpdate, component)
			cm.status.admitted = append(cm.status.admitted, ytv1.ComponentUpdateProgress{
				Component: component,
				Strategy:  updatePlan[index].Strategy.DeepCopy(),
				Phase:     ytv1.ComponentUpdatePhasePending,
			})
		} else {
			cm.status.cannotUpdate = append(cm.status.cannotUpdate, component)
		}
	}
}

func (cm *ComponentManager) initUpdateConditions(ctx context.Context) {
	// Take snapshot of status conditions to change them during update without changing current update workflow.
	conditions := []string{
		consts.ConditionMasterCellsPreparation,
		consts.ConditionMasterCellsRegistration,
		consts.ConditionMasterCellsCompletion,
	}
	for _, name := range conditions {
		if condition := cm.ytsaurus.GetStatusCondition(name); condition != nil {
			cm.ytsaurus.SetUpdateStatusCondition(ctx, *condition)
		}
	}
}

func canUpdateComponent(selector ytv1.ComponentUpdateSelector, component ytv1.Component) bool {
	switch selector.Class {
	case consts.ComponentClassUnspecified:
		if selector.Component.Type == component.Type && (selector.Component.Name == "" || selector.Component.Name == component.Name) {
			return true
		}
	case consts.ComponentClassEverything:
		return true
	case consts.ComponentClassNothing:
		return false
	case consts.ComponentClassStateless:
		switch component.Type {
		case consts.DataNodeType, consts.TabletNodeType, consts.MasterType:
			return false
		default:
			return true
		}
	}
	return false
}
