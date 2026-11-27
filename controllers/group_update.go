package controllers

import (
	"k8s.io/apimachinery/pkg/api/meta"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// Masters and tablet nodes need coordinated safety steps if any admitted group
// of that type uses a strategy other than OnDelete.
func hasNonOnDeleteGroup(groups []ytv1.ComponentUpdateProgress, typ consts.ComponentType) bool {
	for _, group := range groups {
		if group.Type == typ && (group.Strategy == nil || group.Strategy.Type() != ytv1.ComponentUpdateModeTypeOnDelete) {
			return true
		}
	}
	return false
}

// Bind the update to the effective workload inputs. During bulk recreation a
// changed target can be applied at the current phase; removal is never replayed.
func (cm *ComponentManager) refreshGroupTarget(c components.Component) error {
	state := cm.ytsaurus.GetUpdateState()
	if state != ytv1.UpdateStateWaitingForPodsRemoval && state != ytv1.UpdateStateWaitingForPodsCreation &&
		!(state == ytv1.UpdateStateWaitingForMasterReady && c.GetType() == consts.MasterType) {
		return nil
	}
	p := cm.ytsaurus.GetGroupProgress(c.GetComponent())
	if p == nil {
		return nil
	}
	target, err := components.DesiredWorkloadRevision(c)
	if err != nil {
		return err
	}
	if target == p.TargetRevision {
		return nil
	}
	if p.Phase == ytv1.ComponentUpdatePhaseWorkloadStarted && p.TargetRevision != "" {
		// This group finished the admitted target. Admit the new target in the
		// next batch, when the update plan and safety checks run again.
		if cm.deferred == nil {
			cm.deferred = make(map[ytv1.Component]bool)
		}
		cm.deferred[p.Component] = true
		p.Reason = "New workload target pending the next update"
		return nil
	}
	if p.TargetRevision != "" && p.Phase == ytv1.ComponentUpdatePhaseReplacing {
		l := c.GetLabeller()
		conditions := &cm.ytsaurus.GetResource().Status.UpdateStatus.Conditions
		for _, name := range []string{l.GetPodsUpdatedCondition(), l.GetWaitingOnDeleteUpdateCondition(),
			l.GetCondition(consts.ConditionSchedulerJobsEnabled), l.GetReadyCondition()} {
			meta.RemoveStatusCondition(conditions, name)
		}
	}
	p.TargetRevision = target
	cm.targetChanged = true
	return nil
}

func (cm *ComponentManager) startGroupProgress() {
	groups := cm.status.admitted
	cm.ytsaurus.GetResource().Status.UpdateStatus.GroupProgress = groups
	for _, c := range cm.allComponents {
		if p := cm.ytsaurus.GetGroupProgress(c.GetComponent()); p != nil {
			target, err := components.DesiredWorkloadRevision(c)
			if err != nil {
				// Only this group retries rendering when workload reconciliation starts.
				p.Reason = err.Error()
				continue
			}
			p.TargetRevision = target
		}
	}
}

// Checkpoints are written before dispatching the next action. In particular,
// a restarted operator can never issue a scale-down from the Creating phase.
func (cm *ComponentManager) advanceGroupProgress() bool {
	state := cm.ytsaurus.GetUpdateState()
	if state != ytv1.UpdateStateWaitingForPodsRemoval && state != ytv1.UpdateStateWaitingForPodsCreation &&
		state != ytv1.UpdateStateWaitingForMasterReady {
		return false
	}
	changed := false
	for _, c := range cm.allComponents {
		p := cm.ytsaurus.GetGroupProgress(c.GetComponent())
		if p == nil || (state == ytv1.UpdateStateWaitingForMasterReady && c.GetType() != consts.MasterType) {
			continue
		}
		before := p.Phase
		cm.advanceGroup(c, p)
		changed = changed || before != p.Phase
	}
	return changed
}

func (cm *ComponentManager) advanceGroup(c components.Component, p *ytv1.ComponentUpdateProgress) {
	status := c.GetStatus()
	if p.Phase != ytv1.ComponentUpdatePhaseWorkloadStarted {
		p.Reason = status.Message
	}
	switch p.Phase {
	case ytv1.ComponentUpdatePhasePending:
		p.Phase = ytv1.ComponentUpdatePhaseRemoving
		if components.WorkloadReplacementMode(c, p.Strategy) {
			p.Phase = ytv1.ComponentUpdatePhaseReplacing
		}
	case ytv1.ComponentUpdatePhaseRemoving:
		if cm.ytsaurus.IsUpdateStatusConditionTrue(c.GetLabeller().GetPodsRemovedCondition()) {
			p.Phase = ytv1.ComponentUpdatePhaseCreating
			p.Reason = ""
		}
	case ytv1.ComponentUpdatePhaseCreating, ytv1.ComponentUpdatePhaseReplacing:
		state := cm.ytsaurus.GetComponentUpdateState(p.Component)
		if p.Type == consts.MasterType && p.Phase == ytv1.ComponentUpdatePhaseCreating {
			// A bulk master recreates its pods in its dedicated recovery step.
			if state != ytv1.UpdateStateWaitingForMasterReady {
				return
			}
		} else if p.Phase == ytv1.ComponentUpdatePhaseCreating && state != ytv1.UpdateStateWaitingForPodsCreation {
			return
		}
		if p.Phase == ytv1.ComponentUpdatePhaseReplacing &&
			!cm.ytsaurus.IsUpdateStatusConditionTrue(c.GetLabeller().GetPodsUpdatedCondition()) {
			return
		}
		// Use the same Started threshold as the global creation step: some
		// components need a later cluster-wide hook to become fully ready.
		if !status.IsReady() && status.SyncStatus != components.SyncStatusStarted {
			return
		}
		if p.Phase == ytv1.ComponentUpdatePhaseReplacing && p.Type != consts.MasterType {
			// Replacement handlers can return Ready as soon as PodsUpdated is
			// recorded. Run the normal startup checks and any remaining config
			// sync (for example, exec-node sidecars) before joining the batch.
			p.Phase = ytv1.ComponentUpdatePhaseCreating
		} else {
			p.Phase = ytv1.ComponentUpdatePhaseWorkloadStarted
		}
		p.Reason = ""
	}
}

func (cm *ComponentManager) workloadsStarted() bool {
	if !cm.status.allStarted {
		return false
	}
	for _, c := range cm.allComponents {
		progress := cm.ytsaurus.GetGroupProgress(c.GetComponent())
		if progress == nil || progress.Phase == ytv1.ComponentUpdatePhaseWorkloadStarted {
			continue
		}
		// Master-cell maintenance starts masters after the ordinary creation step.
		if cm.status.mastersMaintenance && c.GetType() == consts.MasterType {
			continue
		}
		return false
	}
	return true
}
