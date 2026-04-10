package controllers

import (
	"context"
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
}

type ComponentManagerStatus struct {
	masterReady bool // All masters are Ready - quorum may enter/exit read-only state
	allStarted  bool // All components are Started or Running - no reconciliations required
	allRunning  bool // All components are Ready or NeedUpdate - can start updates
	allReady    bool // All components are Ready - no reconciliations required

	pending []ytv1.Component // Components in state Pending or Updating
	blocked []ytv1.Component // Components in state Blocked
	started []ytv1.Component // Components in state Started

	needUpdate   []ytv1.Component // Components in state NeedUpdate
	canUpdate    []ytv1.Component // Components with update allowed
	cannotUpdate []ytv1.Component // Components with update blocked
	nowUpdating  []ytv1.Component // Components updating right now

	clusterMaintenance bool
	mastersMaintenance bool
	shutdownStorage    bool
	shutdownTablets    bool
	shutdownCompute    bool
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

	var hps []components.Component
	for _, hpSpec := range resource.Spec.HTTPProxies {
		hps = append(hps, components.NewHTTPProxy(cfgen, ytsaurus, m, hpSpec))
	}
	allComponents = append(allComponents, hps...)

	yc := components.NewYtsaurusClient(cfgen, ytsaurus, m, hps[0], getAllComponents)
	allComponents = append(allComponents, yc)

	d := components.NewDiscovery(cfgen, ytsaurus, yc)
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
			tnds = append(tnds, components.NewTabletNode(nodeCfgGen, ytsaurus, yc, tndSpec, idx == 0))
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

	if resource.Spec.PrimaryMasters.Timbertruck != nil {
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
		err := component.Fetch(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch component %s: %w", component.GetFullName(), err)
		}

		// TODO: Reorder logic into less weird sequence.
		var status components.ComponentStatus
		if cm.ytsaurus.IsReadyToUpdate() {
			status = component.NeedUpdate()
		}
		if !status.IsNeedUpdate() {
			status, err = component.Sync(ctx, true)
			if err != nil {
				return fmt.Errorf("failed to get component %s status: %w", component.GetFullName(), err)
			}
		}

		component.SetStatus(status)
		component.SetReadyCondition(status)
		logger.Info("Component status",
			"component", component.GetFullName(),
			"status", status.SyncStatus,
			"message", status.Message,
		)

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
	var syncErr error
	for _, c := range cm.allComponents {
		status, err := c.Sync(ctx, true)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("failed to get status for %s: %w", c.GetFullName(), err)
		}
		c.SetStatus(status)

		if status.SyncStatus == components.SyncStatusPending ||
			status.SyncStatus == components.SyncStatusUpdating {
			hasPending = true
			logger.Info("Sync component",
				"component", c.GetFullName(),
				"status", status.SyncStatus,
				"message", status.Message,
			)
			if status, err := c.Sync(ctx, false); err != nil {
				logger.Error(err, "Cannot sync component",
					"component", c.GetFullName(),
					"status", status.SyncStatus,
					"message", status.Message,
				)
				syncErr = err
				break
			}
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

	if syncErr != nil {
		return ctrl.Result{Requeue: true}, syncErr
	}

	if !hasPending {
		// All components are blocked.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
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
	cm.status.cannotUpdate = nil
	count := make([]int32, len(updatePlan))
	for _, component := range cm.status.needUpdate {
		index := -1
		for i, selector := range updatePlan {
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
		} else {
			cm.status.cannotUpdate = append(cm.status.cannotUpdate, component)
		}
	}
}

func (cm *ComponentManager) initUpdateConditions(ctx context.Context) {
	// Take snapshot of status conditions to change them during update without changing current update workflow.
	if condition := cm.ytsaurus.GetStatusCondition(consts.ConditionMasterCellsRegistration); condition != nil {
		cm.ytsaurus.SetUpdateStatusCondition(ctx, *condition)
	}
	if condition := cm.ytsaurus.GetStatusCondition(consts.ConditionMasterCellsSettlement); condition != nil {
		cm.ytsaurus.SetUpdateStatusCondition(ctx, *condition)
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
