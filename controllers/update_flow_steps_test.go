package controllers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
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
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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
				ytv1.UpdateStateWaitingForImaginaryChunksAbsence,
				ytv1.UpdateStateWaitingForSnapshots,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForMasterExitReadOnly,
				ytv1.UpdateStateWaitingForSidecarsInitializingPrepare,
				ytv1.UpdateStateWaitingForSidecarsInitialize,
				ytv1.UpdateStateWaitingForSafeModeDisabled,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		},
		{
			name: "queue agent update",
			updatingComponents: []ytv1.Component{
				{Type: consts.QueueAgentType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForQAStateUpdatingPrepare,
				ytv1.UpdateStateWaitingForQAStateUpdate,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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
				ytv1.UpdateStateWaitingForImaginaryChunksAbsence,
				ytv1.UpdateStateWaitingForSnapshots,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForMasterExitReadOnly,
				ytv1.UpdateStateWaitingForSidecarsInitializingPrepare,
				ytv1.UpdateStateWaitingForSidecarsInitialize,
				ytv1.UpdateStateWaitingForTabletCellsRecovery,
				ytv1.UpdateStateWaitingForSafeModeDisabled,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
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

			require.Emptyf(t, cmp.Diff(tt.expectedStates, states), "Flow states mismatch")
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
