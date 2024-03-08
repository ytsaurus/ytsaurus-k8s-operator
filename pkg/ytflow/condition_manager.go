package ytflow

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

type conditionManager struct {
	client   client.Client
	ytsaurus *ytv1.Ytsaurus
}

func newConditionManager(client client.Client, ytsaurus *ytv1.Ytsaurus) *conditionManager {
	return &conditionManager{
		client:   client,
		ytsaurus: ytsaurus,
	}
}

func (cm *conditionManager) SetTrue(ctx context.Context, condName conditionName, msg string) error {
	return cm.Set(ctx, condName, true, msg)
}
func (cm *conditionManager) SetFalse(ctx context.Context, condName conditionName, msg string) error {
	return cm.Set(ctx, condName, false, msg)
}
func (cm *conditionManager) Set(ctx context.Context, condName conditionName, val bool, msg string) error {
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
func (cm *conditionManager) IsTrue(condName conditionName) bool {
	return meta.IsStatusConditionTrue(cm.ytsaurus.Status.Conditions, string(condName))
}
func (cm *conditionManager) IsFalse(condName conditionName) bool {
	return !cm.IsTrue(condName)
}
func (cm *conditionManager) Get(condName conditionName) bool {
	if cm.IsTrue(condName) {
		return true
	} else {
		return false
	}
}
func (cm *conditionManager) IsSatisfied(condDep conditionDependency) bool {
	realValue := cm.Get(condDep.name)
	return realValue == condDep.val
}

func (cm *conditionManager) updateStatusRetryOnConflict(ctx context.Context, change func(ytsaurusResource *ytv1.Ytsaurus)) error {
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
