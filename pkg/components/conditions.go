package components

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

type ConditionName string

type Condition struct {
	Name ConditionName
	Val  bool
}

func (c Condition) String() string {
	if c.Val {
		return string(c.Name)
	}
	return fmt.Sprintf("!%s", c.Name)
}

func not(condDep Condition) Condition {
	return Condition{
		Name: condDep.Name,
		Val:  !condDep.Val,
	}
}
func isTrue(cond ConditionName) Condition {
	// '^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$' for conditions.
	replaced := strings.ReplaceAll(string(cond), "-", "_")
	return Condition{Name: ConditionName(replaced), Val: true}
}

// buildFinished means that component was fully built initially.
func buildStarted(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sBuildStarted", compName)))
}
func buildFinished(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sBuildFinished", compName)))
}

func initializationStarted(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%snitializationStarted", compName)))
}
func initializationFinished(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%snitializationFinished", compName)))
}
func updateRequired(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sUpdateRequired", compName)))
}
func rebuildStarted(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sRebuildStarted", compName)))
}
func podsRemoved(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sPodsRemoved", compName)))
}
func podsCreated(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sPodsCreated", compName)))
}
func rebuildFinished(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sRebuildFinished", compName)))
}

type baseStateManager struct {
	client   client.Client
	ytsaurus *ytv1.Ytsaurus
}

type ConditionManager struct {
	baseStateManager
}
type StateManager struct {
	baseStateManager
}

func NewConditionManagerFromYtsaurus(ytsaurus *apiproxy.Ytsaurus) *ConditionManager {
	return NewConditionManager(
		ytsaurus.APIProxy().Client(),
		ytsaurus.GetResource(),
	)
}

func NewConditionManager(client client.Client, ytsaurus *ytv1.Ytsaurus) *ConditionManager {
	return &ConditionManager{
		baseStateManager{
			client:   client,
			ytsaurus: ytsaurus,
		},
	}
}

func NewStateManagerFromYtsaurus(ytsaurus *apiproxy.Ytsaurus) *StateManager {
	return NewStateManager(
		ytsaurus.APIProxy().Client(),
		ytsaurus.GetResource(),
	)
}

func NewStateManager(client client.Client, ytsaurus *ytv1.Ytsaurus) *StateManager {
	return &StateManager{
		baseStateManager{
			client:   client,
			ytsaurus: ytsaurus,
		},
	}
}

// TODO: refactor this for managers to receive interface which can persist state on resource
// to reuse for other CRDS & don't have such wide interfaces for managers.
func (m *baseStateManager) updateStatusRetryOnConflict(ctx context.Context, change func(ytsaurusResource *ytv1.Ytsaurus)) error {
	//tryUpdate := func(ytsaurus *ytv1.Ytsaurus) error {
	//	// N.B. Status().Update(...) updates not only status sub-resource, but also main
	//	// ytsaurus resource which is the same reference in a lot of places of our code,
	//	// and it bites us, because we're modifying spec (monitoring port).
	//	// Not sure if it is good workaround, need to think about it,
	//	// but we should get rid of overriding port and possibly using references to the main resource.
	//	ytsaurusTemp := &ytv1.Ytsaurus{
	//		ObjectMeta: metav1.ObjectMeta{
	//			Name:            ytsaurus.Name,
	//			Namespace:       ytsaurus.Namespace,
	//			ResourceVersion: ytsaurus.ResourceVersion,
	//		},
	//		Status: ytsaurus.Status,
	//	}
	//	change(ytsaurusTemp)
	//
	//	err := m.client.Status().Update(ctx, ytsaurusTemp)
	//	m.ytsaurus.Status = ytsaurusTemp.Status
	//	// You have to return err itself here (not wrapped inside another error)
	//	// so that RetryOnConflict can identify it correctly.
	//	return err
	//}

	change(m.ytsaurus)
	return m.client.Status().Update(ctx, m.ytsaurus)
	//err := tryUpdate(m.ytsaurus)
	//if err == nil || !apierrors.IsConflict(err) {
	//	return err
	//}
	//
	//return retry.RetryOnConflict(retry.DefaultRetry, func() error {
	//	// Fetch the resource here; you need to refetch it on every try, since
	//	// if you got a conflict on the last update attempt then you need to get
	//	// the current version before making your own changes.
	//	ytsaurus := ytv1.Ytsaurus{}
	//	name := types.NamespacedName{
	//		Namespace: m.ytsaurus.Namespace,
	//		Name:      m.ytsaurus.Name,
	//	}
	//	if err = m.client.Get(ctx, name, &ytsaurus); err != nil {
	//		return err
	//	}
	//
	//	return tryUpdate(&ytsaurus)
	//})
}

func (cm *ConditionManager) SetTrue(ctx context.Context, condName ConditionName) error {
	return cm.SetTrueMsg(ctx, condName, "")
}
func (cm *ConditionManager) SetTrueMsg(ctx context.Context, condName ConditionName, msg string) error {
	return cm.SetMsg(ctx, condName, true, msg)
}
func (cm *ConditionManager) SetFalse(ctx context.Context, condName ConditionName) error {
	return cm.SetFalseMsg(ctx, condName, "")
}
func (cm *ConditionManager) SetFalseMsg(ctx context.Context, condName ConditionName, msg string) error {
	return cm.SetMsg(ctx, condName, false, msg)
}
func (cm *ConditionManager) Set(ctx context.Context, condName ConditionName, val bool) error {
	return cm.SetMsg(ctx, condName, val, "")
}
func (cm *ConditionManager) SetCond(ctx context.Context, cond Condition) error {
	return cm.SetMsg(ctx, cond.Name, cond.Val, "")
}
func (cm *ConditionManager) SetCondMany(ctx context.Context, conds ...Condition) error {
	var metaconds []metav1.Condition
	for _, cond := range conds {
		metaconds = append(metaconds, cm.buildCond(cond.Name, cond.Val, ""))
	}
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		for _, metacond := range metaconds {
			meta.SetStatusCondition(&ytsaurus.Status.Conditions, metacond)
		}
	})
}
func (cm *ConditionManager) SetCondMsg(ctx context.Context, cond Condition, msg string) error {
	return cm.SetMsg(ctx, cond.Name, cond.Val, msg)
}
func (cm *ConditionManager) buildCond(condName ConditionName, val bool, msg string) metav1.Condition {
	return metav1.Condition{
		Type: string(condName),
		Status: map[bool]metav1.ConditionStatus{
			true:  metav1.ConditionTrue,
			false: metav1.ConditionFalse,
		}[val],
		// DO we need better reason?
		Reason:  string(condName),
		Message: msg,
	}
}
func (cm *ConditionManager) SetMsg(ctx context.Context, condName ConditionName, val bool, msg string) error {
	metacond := cm.buildCond(condName, val, msg)
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		meta.SetStatusCondition(&ytsaurus.Status.Conditions, metacond)
	})
}
func (cm *ConditionManager) IsTrue(condName ConditionName) bool {
	return meta.IsStatusConditionTrue(cm.ytsaurus.Status.Conditions, string(condName))
}
func (cm *ConditionManager) IsFalse(condName ConditionName) bool {
	return !cm.IsTrue(condName)
}
func (cm *ConditionManager) Is(cond Condition) bool {
	return cm.IsSatisfied(cond)
}
func (cm *ConditionManager) All(conds ...Condition) bool {
	for _, cond := range conds {
		if cm.IsNotSatisfied(cond) {
			return false
		}
	}
	return true
}
func (cm *ConditionManager) Any(conds ...Condition) bool {
	for _, cond := range conds {
		if cm.IsSatisfied(cond) {
			return true
		}
	}
	return false
}
func (cm *ConditionManager) IsSatisfied(cond Condition) bool {
	return cm.IsTrue(cond.Name) == cond.Val
}
func (cm *ConditionManager) IsNotSatisfied(cond Condition) bool {
	return !cm.IsSatisfied(cond)
}
func (cm *ConditionManager) Get(condName ConditionName) bool {
	if cm.IsTrue(condName) {
		return true
	} else {
		return false
	}
}

func (cm *StateManager) SetClusterState(ctx context.Context, clusterState ytv1.ClusterState) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.State = clusterState
	})
}
func (cm *StateManager) SetClusterUpdateState(ctx context.Context, updateState ytv1.UpdateState) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.UpdateStatus.State = updateState
	})
}
func (cm *StateManager) SetTabletCellBundles(ctx context.Context, cells []ytv1.TabletCellBundleInfo) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.UpdateStatus.TabletCellBundles = cells
	})
}
func (cm *StateManager) SetMasterMonitoringPaths(ctx context.Context, paths []string) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.UpdateStatus.MasterMonitoringPaths = paths
	})
}
func (cm *StateManager) GetTabletCellBundles() []ytv1.TabletCellBundleInfo {
	return cm.ytsaurus.Status.UpdateStatus.TabletCellBundles
}
func (cm *StateManager) GetMasterMonitoringPaths() []string {
	return cm.ytsaurus.Status.UpdateStatus.MasterMonitoringPaths
}
