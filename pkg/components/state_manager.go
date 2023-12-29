package components

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

// ytsaurusResourceStateManager is a set of methods to get and set ytsaurusStateManager k8s CRD state.
// It is introduced to be able to have dummy implementation for remote components.
// Not sure if it is cool to kinda mock behaviour instead of explicitly deal with
// local or remote ytsaurusStateManager, but currently it is really deep in all components, so let's see how it'll go.
type ytsaurusResourceStateManager interface {
	GetClusterState() ytv1.ClusterState
	GetUpdateState() ytv1.UpdateState
	SetStatusCondition(metav1.Condition)
	SetUpdateStatusCondition(metav1.Condition)
	IsUpdateStatusConditionTrue(string) bool
	IsStatusConditionTrue(string) bool

	// TODO: not sure if we need that
	GetLocalUpdatingComponents() []string
}

type RemoteYtsaurusStateManager struct{}

func NewRemoteYtsaurusStateManager() *RemoteYtsaurusStateManager {
	return &RemoteYtsaurusStateManager{}
}

func (m *RemoteYtsaurusStateManager) GetClusterState() ytv1.ClusterState {
	return ytv1.ClusterStateRunning
}

func (m *RemoteYtsaurusStateManager) GetUpdateState() ytv1.UpdateState {
	return ytv1.UpdateStateNone
}

func (m *RemoteYtsaurusStateManager) SetStatusCondition(metav1.Condition) {}

func (m *RemoteYtsaurusStateManager) GetLocalUpdatingComponents() []string {
	return []string{}
}

func (m *RemoteYtsaurusStateManager) SetUpdateStatusCondition(metav1.Condition) {}

func (m *RemoteYtsaurusStateManager) IsUpdateStatusConditionTrue(string) bool { return true }

func (m *RemoteYtsaurusStateManager) IsStatusConditionTrue(string) bool { return true }
