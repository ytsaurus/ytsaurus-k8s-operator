package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

func TestBuildFlowTree(t *testing.T) {
	fullComponents := []ytv1.Component{
		{ComponentType: consts.MasterType},
		{ComponentType: consts.TabletNodeType},
		{ComponentType: consts.SchedulerType},
		{ComponentType: consts.QueryTrackerType},
		{ComponentType: consts.YqlAgentType},
	}

	masterTabletComponents := []ytv1.Component{
		{ComponentType: consts.MasterType},
		{ComponentType: consts.TabletNodeType},
	}

	onlyMasterComponents := []ytv1.Component{
		{ComponentType: consts.MasterType},
	}

	tests := []struct {
		name               string
		updatingComponents []ytv1.Component
		allComponents      []ytv1.Component
		expectedStates     []ytv1.UpdateState
		unhappyPath        bool
	}{
		{
			name:               "empty updating components with full components",
			updatingComponents: []ytv1.Component{},
			allComponents:      fullComponents,
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
				ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare,
				ytv1.UpdateStateWaitingForOpArchiveUpdate,
				ytv1.UpdateStateWaitingForQTStateUpdatingPrepare,
				ytv1.UpdateStateWaitingForQTStateUpdate,
				ytv1.UpdateStateWaitingForYqlaUpdatingPrepare,
				ytv1.UpdateStateWaitingForYqlaUpdate,
				ytv1.UpdateStateWaitingForSafeModeDisabled,
			},
		},
		{
			name:               "empty updating components with full components - unhappy path",
			updatingComponents: []ytv1.Component{},
			allComponents:      fullComponents,
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStatePossibilityCheck,
				ytv1.UpdateStateImpossibleToStart,
			},
			unhappyPath: true,
		},
		{
			name:               "empty updating components with master-tablet only",
			updatingComponents: []ytv1.Component{},
			allComponents:      masterTabletComponents,
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
		{
			name:               "empty updating components with master only",
			updatingComponents: []ytv1.Component{},
			allComponents:      onlyMasterComponents,
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
			name: "master update with full components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.MasterType},
			},
			allComponents: fullComponents,
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
			name: "tablet update with full components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.TabletNodeType},
			},
			allComponents: fullComponents,
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
			name: "scheduler update with full components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.SchedulerType},
			},
			allComponents: fullComponents,
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare,
				ytv1.UpdateStateWaitingForOpArchiveUpdate,
			},
		},
		{
			name: "query tracker update with full components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.QueryTrackerType},
			},
			allComponents: fullComponents,
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForQTStateUpdatingPrepare,
				ytv1.UpdateStateWaitingForQTStateUpdate,
			},
		},
		{
			name: "yql agent update with full components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.YqlAgentType},
			},
			allComponents: fullComponents,
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForYqlaUpdatingPrepare,
				ytv1.UpdateStateWaitingForYqlaUpdate,
			},
		},
		{
			name: "random stateless component update with full components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.DiscoveryType},
			},
			allComponents: fullComponents,
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
			},
		},
		{
			name: "combined master and tablet update with full components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.MasterType},
				{ComponentType: consts.TabletNodeType},
			},
			allComponents: fullComponents,
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
		{
			name: "master update with master-tablet components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.MasterType},
			},
			allComponents: masterTabletComponents,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := buildFlowTree(tt.updatingComponents, tt.allComponents)

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
	allComponents := []ytv1.Component{
		{ComponentType: consts.MasterType},
		{ComponentType: consts.TabletNodeType},
	}

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
			expected:           true,
		},
		{
			name:               "empty updating components",
			updatingComponents: []ytv1.Component{},
			componentType:      consts.MasterType,
			expected:           true,
		},
		{
			name: "component present in updating components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.MasterType},
			},
			componentType: consts.MasterType,
			expected:      true,
		},
		{
			name: "component not present in updating components",
			updatingComponents: []ytv1.Component{
				{ComponentType: consts.TabletNodeType},
			},
			componentType: consts.MasterType,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasComponent(tt.updatingComponents, allComponents, tt.componentType)
			require.Equal(t, tt.expected, result)
		})
	}
}
