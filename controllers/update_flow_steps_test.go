package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

func TestBuildFlowTree(t *testing.T) {
	tests := []struct {
		name               string
		updatingComponents []ytv1.Component
		expectedStates     []ytv1.UpdateState
		unhappyPath        bool
	}{
		{
			name:               "empty updating components",
			updatingComponents: []ytv1.Component{},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
			},
		},
		{
			name: "master update",
			updatingComponents: []ytv1.Component{
				{Type: consts.MasterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStatePossibilityCheck,
				ytv1.UpdateStateWaitingForSafeModeEnabled,
				ytv1.UpdateStateWaitingForEnableRealChunkLocations,
				ytv1.UpdateStateWaitingForSnapshots,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForMasterExitReadOnly,
				ytv1.UpdateStateWaitingForSafeModeDisabled,
			},
		},
		{
			name: "tablet update",
			updatingComponents: []ytv1.Component{
				{Type: consts.TabletNodeType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStatePossibilityCheck,
				ytv1.UpdateStateWaitingForTabletCellsSaving,
				ytv1.UpdateStateWaitingForTabletCellsRemovingStart,
				ytv1.UpdateStateWaitingForTabletCellsRemoved,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForTabletCellsRecovery,
			},
		},
		{
			name: "scheduler update",
			updatingComponents: []ytv1.Component{
				{Type: consts.SchedulerType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare,
				ytv1.UpdateStateWaitingForOpArchiveUpdate,
			},
		},
		{
			name: "query tracker update",
			updatingComponents: []ytv1.Component{
				{Type: consts.QueryTrackerType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForQTStateUpdatingPrepare,
				ytv1.UpdateStateWaitingForQTStateUpdate,
			},
		},
		{
			name: "yql agent update",
			updatingComponents: []ytv1.Component{
				{Type: consts.YqlAgentType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForYqlaUpdatingPrepare,
				ytv1.UpdateStateWaitingForYqlaUpdate,
			},
		},
		{
			name: "random stateless component update",
			updatingComponents: []ytv1.Component{
				{Type: consts.DiscoveryType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
			},
		},
		{
			name: "combined master and tablet update",
			updatingComponents: []ytv1.Component{
				{Type: consts.MasterType},
				{Type: consts.TabletNodeType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStatePossibilityCheck,
				ytv1.UpdateStateWaitingForSafeModeEnabled,
				ytv1.UpdateStateWaitingForTabletCellsSaving,
				ytv1.UpdateStateWaitingForTabletCellsRemovingStart,
				ytv1.UpdateStateWaitingForTabletCellsRemoved,
				ytv1.UpdateStateWaitingForEnableRealChunkLocations,
				ytv1.UpdateStateWaitingForSnapshots,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForMasterExitReadOnly,
				ytv1.UpdateStateWaitingForTabletCellsRecovery,
				ytv1.UpdateStateWaitingForSafeModeDisabled,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := buildFlowTree(tt.updatingComponents)

			// Collect all states from the tree
			var states []ytv1.UpdateState
			currentStep := tree.head
			for currentStep != nil {
				states = append(states, currentStep.updateState)
				if tt.unhappyPath && currentStep.updateState == ytv1.UpdateStatePossibilityCheck {
					currentStep = currentStep.nextSteps[stepResultMarkUnhappy]
				} else {
					currentStep = currentStep.nextSteps[stepResultMarkHappy]
				}
			}

			require.Equal(t, tt.expectedStates, states, "Flow states mismatch")
		})
	}
}

func TestHasComponent(t *testing.T) {
	tests := []struct {
		name               string
		updatingComponents []ytv1.Component
		componentType      consts.ComponentType
		expected           bool
	}{
		{
			name:               "nil updating components",
			updatingComponents: nil,
			componentType:      consts.MasterType,
			expected:           false,
		},
		{
			name:               "empty updating components",
			updatingComponents: []ytv1.Component{},
			componentType:      consts.MasterType,
			expected:           false,
		},
		{
			name: "component present in updating components",
			updatingComponents: []ytv1.Component{
				{Type: consts.MasterType},
			},
			componentType: consts.MasterType,
			expected:      true,
		},
		{
			name: "component not present in updating components",
			updatingComponents: []ytv1.Component{
				{Type: consts.TabletNodeType},
			},
			componentType: consts.MasterType,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasComponent(tt.updatingComponents, tt.componentType)
			require.Equal(t, tt.expected, result)
		})
	}
}
