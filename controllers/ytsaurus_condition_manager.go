package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

type updateConditionsManager struct {
	ytsaurusProxy *apiProxy.Ytsaurus
}

func newUpdateConditionsManager(ytsaurusProxy *apiProxy.Ytsaurus) *updateConditionsManager {
	return &updateConditionsManager{
		ytsaurusProxy: ytsaurusProxy,
	}
}

func (m *updateConditionsManager) Put(conditionType string) {
	resource := m.ytsaurusProxy.GetResource()
	meta.SetStatusCondition(&resource.Status.UpdateStatus.Conditions, metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: conditionType,
	})
}

func (m *updateConditionsManager) Exists(conditionType string) bool {
	return m.ytsaurusProxy.IsUpdateStatusConditionTrue(conditionType)
}

func (m *updateConditionsManager) Clear(ctx context.Context) error {
	// TODO(l0kix2): here we are clearing all the update fields
	// this may unexpected in some cases.
	return m.ytsaurusProxy.ClearUpdateStatus(ctx)
}
