package state

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

// Condition is what we put in the `Type` field of k8s Condition struct.
// For the sake of brevity we call it just Condition here, but it is really id/name.
// Condition *value* can be true or false.
type Condition string

type Manager struct {
	client   client.Client
	ytsaurus *ytv1.Ytsaurus
}

func NewStateManager(client client.Client, ytsaurus *ytv1.Ytsaurus) *Manager {
	return &Manager{
		client:   client,
		ytsaurus: ytsaurus,
	}
}

func (cm *Manager) SetTrue(ctx context.Context, condName Condition, msg string) error {
	return cm.Set(ctx, condName, true, msg)
}
func (cm *Manager) SetFalse(ctx context.Context, condName Condition, msg string) error {
	return cm.Set(ctx, condName, false, msg)
}
func (cm *Manager) Set(ctx context.Context, condName Condition, val bool, msg string) error {
	metacond := metav1.Condition{
		Type: string(condName),
		Status: map[bool]metav1.ConditionStatus{
			true:  metav1.ConditionTrue,
			false: metav1.ConditionFalse,
		}[val],
		// DO we need better reason?
		Reason:  string(condName),
		Message: msg,
	}
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		meta.SetStatusCondition(&ytsaurus.Status.Conditions, metacond)
	})
}
func (cm *Manager) IsTrue(condName Condition) bool {
	return meta.IsStatusConditionTrue(cm.ytsaurus.Status.Conditions, string(condName))
}
func (cm *Manager) IsFalse(condName Condition) bool {
	return !cm.IsTrue(condName)
}
func (cm *Manager) Get(condName Condition) bool {
	if cm.IsTrue(condName) {
		return true
	} else {
		return false
	}
}

func (cm *Manager) SetClusterState(ctx context.Context, clusterState ytv1.ClusterState) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.State = clusterState
	})
}
func (cm *Manager) SetTabletCellBundles(ctx context.Context, cells []ytv1.TabletCellBundleInfo) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.UpdateStatus.TabletCellBundles = cells
	})
}
func (cm *Manager) SetMasterMonitoringPaths(ctx context.Context, paths []string) error {
	return cm.updateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		ytsaurus.Status.UpdateStatus.MasterMonitoringPaths = paths
	})
}
func (cm *Manager) GetClusterState() ytv1.ClusterState {
	return cm.ytsaurus.Status.State
}
func (cm *Manager) GetTabletCellBundles() []ytv1.TabletCellBundleInfo {
	return cm.ytsaurus.Status.UpdateStatus.TabletCellBundles
}
func (cm *Manager) GetMasterMonitoringPaths() []string {
	return cm.ytsaurus.Status.UpdateStatus.MasterMonitoringPaths
}

func (cm *Manager) updateStatusRetryOnConflict(ctx context.Context, change func(ytsaurusResource *ytv1.Ytsaurus)) error {
	tryUpdate := func(ytsaurus *ytv1.Ytsaurus) error {
		change(ytsaurus)
		// You have to return err itself here (not wrapped inside another error)
		// so that RetryOnConflict can identify it correctly.
		err := cm.client.Status().Update(ctx, ytsaurus)
		if err == nil {
			cm.ytsaurus = ytsaurus
		}
		return err
	}

	err := tryUpdate(cm.ytsaurus)
	if err == nil || !errors.IsConflict(err) {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the resource here; you need to refetch it on every try, since
		// if you got a conflict on the last update attempt then you need to get
		// the current version before making your own changes.
		ytsaurus := ytv1.Ytsaurus{}
		name := types.NamespacedName{
			Namespace: cm.ytsaurus.Namespace,
			Name:      cm.ytsaurus.Name,
		}
		if err = cm.client.Get(ctx, name, &ytsaurus); err != nil {
			return err
		}

		return tryUpdate(&ytsaurus)
	})
}
