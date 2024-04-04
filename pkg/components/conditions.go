package components

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
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
	replaced := strings.Replace(string(cond), "-", "_", -1)
	return Condition{Name: ConditionName(replaced), Val: true}
}

// buildFinished means that component was fully built initally.
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
func rebuildFinished(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sRebuildFinished", compName)))
}

type baseStateManager struct {
	client   client.Client
	ytsaurus *ytv1.Ytsaurus
}

type conditionManager struct {
	baseStateManager
}
type stateManager struct {
	baseStateManager
}

func newConditionManagerFromYtsaurus(ytsaurus *apiproxy.Ytsaurus) *conditionManager {
	return newConditionManager(
		ytsaurus.APIProxy().Client(),
		ytsaurus.GetResource(),
	)
}

func newConditionManager(client client.Client, ytsaurus *ytv1.Ytsaurus) *conditionManager {
	return &conditionManager{
		baseStateManager{
			client:   client,
			ytsaurus: ytsaurus,
		},
	}
}

func newStateManagerFromYtsaurus(ytsaurus *apiproxy.Ytsaurus) *stateManager {
	return newStateManager(
		ytsaurus.APIProxy().Client(),
		ytsaurus.GetResource(),
	)
}

func newStateManager(client client.Client, ytsaurus *ytv1.Ytsaurus) *stateManager {
	return &stateManager{
		baseStateManager{
			client:   client,
			ytsaurus: ytsaurus,
		},
	}
}

func (m *baseStateManager) updateStatusRetryOnConflict(ctx context.Context, change func(ytsaurusResource *ytv1.Ytsaurus)) error {
	tryUpdate := func(ytsaurus *ytv1.Ytsaurus) error {
		change(ytsaurus)
		// You have to return err itself here (not wrapped inside another error)
		// so that RetryOnConflict can identify it correctly.
		// N.B. this updates not only status sub-resource but also the main resource.
		err := m.client.Status().Update(ctx, ytsaurus)
		return err
	}

	err := tryUpdate(m.ytsaurus)
	if err == nil || !errors.IsConflict(err) {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the resource here; you need to refetch it on every try, since
		// if you got a conflict on the last update attempt then you need to get
		// the current version before making your own changes.
		ytsaurus := ytv1.Ytsaurus{}
		name := types.NamespacedName{
			Namespace: m.ytsaurus.Namespace,
			Name:      m.ytsaurus.Name,
		}
		if err = m.client.Get(ctx, name, &ytsaurus); err != nil {
			return err
		}

		return tryUpdate(&ytsaurus)
	})
}

func (cm *conditionManager) SetTrue(ctx context.Context, condName ConditionName) error {
	return cm.SetTrueMsg(ctx, condName, "")
}
func (cm *conditionManager) SetTrueMsg(ctx context.Context, condName ConditionName, msg string) error {
	return cm.SetMsg(ctx, condName, true, msg)
}
func (cm *conditionManager) SetFalse(ctx context.Context, condName ConditionName) error {
	return cm.SetFalseMsg(ctx, condName, "")
}
func (cm *conditionManager) SetFalseMsg(ctx context.Context, condName ConditionName, msg string) error {
	return cm.SetMsg(ctx, condName, false, msg)
}
func (cm *conditionManager) Set(ctx context.Context, condName ConditionName, val bool) error {
	return cm.SetMsg(ctx, condName, val, "")
}
func (cm *conditionManager) SetCond(ctx context.Context, cond Condition) error {
	return cm.SetMsg(ctx, cond.Name, cond.Val, "")
}
func (cm *conditionManager) SetCondMany(ctx context.Context, conds ...Condition) error {
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
func (cm *conditionManager) SetCondMsg(ctx context.Context, cond Condition, msg string) error {
	return cm.SetMsg(ctx, cond.Name, cond.Val, msg)
}
func (cm *conditionManager) buildCond(condName ConditionName, val bool, msg string) metav1.Condition {
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
func (cm *conditionManager) SetMsg(ctx context.Context, condName ConditionName, val bool, msg string) error {
	metacond := cm.buildCond(condName, val, msg)
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		meta.SetStatusCondition(&ytsaurus.Status.Conditions, metacond)
	})
}
func (cm *conditionManager) IsTrue(condName ConditionName) bool {
	return meta.IsStatusConditionTrue(cm.ytsaurus.Status.Conditions, string(condName))
}
func (cm *conditionManager) IsFalse(condName ConditionName) bool {
	return !cm.IsTrue(condName)
}
func (cm *conditionManager) Is(cond Condition) bool {
	return cm.IsSatisfied(cond)
}
func (cm *conditionManager) All(conds ...Condition) bool {
	for _, cond := range conds {
		if cm.IsNotSatisfied(cond) {
			return false
		}
	}
	return true
}
func (cm *conditionManager) Any(conds ...Condition) bool {
	for _, cond := range conds {
		if cm.IsSatisfied(cond) {
			return true
		}
	}
	return false
}
func (cm *conditionManager) IsSatisfied(cond Condition) bool {
	return cm.IsTrue(cond.Name) == cond.Val
}
func (cm *conditionManager) IsNotSatisfied(cond Condition) bool {
	return !cm.IsSatisfied(cond)
}
func (cm *conditionManager) Get(condName ConditionName) bool {
	if cm.IsTrue(condName) {
		return true
	} else {
		return false
	}
}

func (cm *stateManager) SetClusterState(ctx context.Context, clusterState ytv1.ClusterState) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.State = clusterState
	})
}
func (cm *stateManager) SetTabletCellBundles(ctx context.Context, cells []ytv1.TabletCellBundleInfo) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.UpdateStatus.TabletCellBundles = cells
	})
}
func (cm *stateManager) SetMasterMonitoringPaths(ctx context.Context, paths []string) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.UpdateStatus.MasterMonitoringPaths = paths
	})
}
func (cm *stateManager) GetTabletCellBundles() []ytv1.TabletCellBundleInfo {
	return cm.ytsaurus.Status.UpdateStatus.TabletCellBundles
}
func (cm *stateManager) GetMasterMonitoringPaths() []string {
	return cm.ytsaurus.Status.UpdateStatus.MasterMonitoringPaths
}
