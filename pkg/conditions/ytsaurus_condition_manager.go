package conditions

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

type ConditionManager struct {
	client   client.Client
	ytsaurus *ytv1.Ytsaurus
}

func NewConditionManager(client client.Client, ytsaurus *ytv1.Ytsaurus) *ConditionManager {
	return &ConditionManager{
		client:   client,
		ytsaurus: ytsaurus,
	}
}

func (cm *ConditionManager) SetTrue(ctx context.Context, condName Condition, msg string) error {
	return cm.Set(ctx, condName, true, msg)
}
func (cm *ConditionManager) SetFalse(ctx context.Context, condName Condition, msg string) error {
	return cm.Set(ctx, condName, false, msg)
}
func (cm *ConditionManager) Set(ctx context.Context, condName Condition, val bool, msg string) error {
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
	return cm.UpdateStatusRetryOnConflict(ctx, func(ytsaurus *ytv1.Ytsaurus) {
		meta.SetStatusCondition(&ytsaurus.Status.Conditions, metacond)
	})
}
func (cm *ConditionManager) IsTrue(condName Condition) bool {
	return meta.IsStatusConditionTrue(cm.ytsaurus.Status.Conditions, string(condName))
}
func (cm *ConditionManager) IsFalse(condName Condition) bool {
	return !cm.IsTrue(condName)
}
func (cm *ConditionManager) Get(condName Condition) bool {
	if cm.IsTrue(condName) {
		return true
	} else {
		return false
	}
}

func (cm *ConditionManager) UpdateStatusRetryOnConflict(ctx context.Context, change func(ytsaurusResource *ytv1.Ytsaurus)) error {
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
