package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type ytsaurusState struct {
	comps          componentsStore
	ytsaurusStatus ytv1.YtsaurusStatus
	statuses       map[string]components.ComponentStatus
}

func newYtsaurusState(
	comps componentsStore,
	ytsaurusStatus ytv1.YtsaurusStatus,

) *ytsaurusState {
	return &ytsaurusState{
		comps:          comps,
		ytsaurusStatus: ytsaurusStatus,
		statuses:       make(map[string]components.ComponentStatus),
	}
}

func (s *ytsaurusState) Build(ctx context.Context) error {
	for _, comp := range s.comps.all2() {
		err := comp.Fetch(ctx)
		name := comp.GetName()
		if err != nil {
			return fmt.Errorf("failed to fetch component %s: %w", name, err)
		}
		status, err := comp.Status2(ctx)
		if err != nil {
			return fmt.Errorf("failed to fetch component's status %s: %w", name, err)
		}
		s.statuses[name] = status
	}
	return nil
}

func (s *ytsaurusState) getMasterStatus() components.ComponentStatus {
	// TODO: constants
	return s.statuses["Master"]
}

func (s *ytsaurusState) isStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(s.ytsaurusStatus.Conditions, conditionType)
}
func (s *ytsaurusState) isStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(s.ytsaurusStatus.Conditions, conditionType)
}
func (s *ytsaurusState) SetStatusCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&s.ytsaurusStatus.Conditions, condition)
}

func (s *ytsaurusState) SetUpdateStatusCondition(condition metav1.Condition) {
	meta.SetStatusCondition(&s.ytsaurusStatus.UpdateStatus.Conditions, condition)
}
