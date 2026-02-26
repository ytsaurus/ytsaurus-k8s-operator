package controllers

import (
	"github.com/google/go-cmp/cmp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

var _ = Describe("BuildFlowTree", func() {
	type testCase struct {
		name               string
		updatingComponents []ytv1.Component
		expectedStates     []ytv1.UpdateState
		unhappyPath        bool
		stopAfterHeated    bool
	}

	DescribeTable("should build correct flow tree",
		func(tc testCase) {
			componentManager := ComponentManager{
				status: ComponentManagerStatus{
					nowUpdating: tc.updatingComponents,
				},
			}
			tree := buildFlowTree(&componentManager)

			// Collect all states from the tree
			var states []ytv1.UpdateState
			currentStep := tree.head
			for currentStep != nil {
				states = append(states, currentStep.updateState)
				if tc.unhappyPath && currentStep.updateState == ytv1.UpdateStatePossibilityCheck {
					currentStep = currentStep.nextSteps[stepResultMarkUnhappy]
				} else if tc.stopAfterHeated && currentStep.updateState == ytv1.UpdateStateWaitingForImageHeater {
					currentStep = currentStep.nextSteps[stepResultMarkUnhappy]
				} else {
					currentStep = currentStep.nextSteps[stepResultMarkHappy]
				}
			}

			Expect(cmp.Diff(tc.expectedStates, states)).To(BeEmpty(), "Flow states mismatch")
		},
		Entry("empty updating components", testCase{
			name:               "empty updating components",
			updatingComponents: []ytv1.Component{},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("master update", testCase{
			name: "master update",
			updatingComponents: []ytv1.Component{
				{Type: consts.MasterType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStatePossibilityCheck,
				ytv1.UpdateStateWaitingForSafeModeEnabled,
				ytv1.UpdateStateWaitingForImaginaryChunksAbsence,
				ytv1.UpdateStateWaitingForSnapshots,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForMasterExitReadOnly,
				ytv1.UpdateStateWaitingForSidecarsInitializingPrepare,
				ytv1.UpdateStateWaitingForSidecarsInitialize,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForSafeModeDisabled,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("tablet update", testCase{
			name: "tablet update",
			updatingComponents: []ytv1.Component{
				{Type: consts.TabletNodeType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStatePossibilityCheck,
				ytv1.UpdateStateWaitingForBundleControllerSaved,
				ytv1.UpdateStateWaitingForBundleControllerDisabled,
				ytv1.UpdateStateWaitingForTabletCellsSaving,
				ytv1.UpdateStateWaitingForTabletCellsRemovingStart,
				ytv1.UpdateStateWaitingForTabletCellsRemoved,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForTabletCellsRecovery,
				ytv1.UpdateStateWaitingForBundleControllerRecovery,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("scheduler update", testCase{
			name: "scheduler update",
			updatingComponents: []ytv1.Component{
				{Type: consts.SchedulerType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare,
				ytv1.UpdateStateWaitingForOpArchiveUpdate,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("query tracker update", testCase{
			name: "query tracker update",
			updatingComponents: []ytv1.Component{
				{Type: consts.QueryTrackerType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForQTStateUpdatingPrepare,
				ytv1.UpdateStateWaitingForQTStateUpdate,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("yql agent update", testCase{
			name: "yql agent update",
			updatingComponents: []ytv1.Component{
				{Type: consts.YqlAgentType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForYqlaUpdatingPrepare,
				ytv1.UpdateStateWaitingForYqlaUpdate,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("queue agent update", testCase{
			name: "queue agent update",
			updatingComponents: []ytv1.Component{
				{Type: consts.QueueAgentType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForQAStateUpdatingPrepare,
				ytv1.UpdateStateWaitingForQAStateUpdate,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("random stateless component update", testCase{
			name: "random stateless component update",
			updatingComponents: []ytv1.Component{
				{Type: consts.DiscoveryType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("combined master and tablet update", testCase{
			name: "combined master and tablet update",
			updatingComponents: []ytv1.Component{
				{Type: consts.MasterType},
				{Type: consts.TabletNodeType},
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
				ytv1.UpdateStatePossibilityCheck,
				ytv1.UpdateStateWaitingForSafeModeEnabled,
				ytv1.UpdateStateWaitingForBundleControllerSaved,
				ytv1.UpdateStateWaitingForBundleControllerDisabled,
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
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForTabletCellsRecovery,
				ytv1.UpdateStateWaitingForBundleControllerRecovery,
				ytv1.UpdateStateWaitingForSafeModeDisabled,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("stateless update without image heater", testCase{
			name: "stateless update without image heater",
			updatingComponents: []ytv1.Component{
				{Type: consts.DiscoveryType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForPodsRemoval,
				ytv1.UpdateStateWaitingForPodsCreation,
				ytv1.UpdateStateWaitingForCypressPatch,
				ytv1.UpdateStateWaitingForTimbertruckPrepared,
			},
		}),
		Entry("image heater only update", testCase{
			name:            "image heater only update",
			stopAfterHeated: true,
			updatingComponents: []ytv1.Component{
				{Type: consts.ImageHeaterType},
			},
			expectedStates: []ytv1.UpdateState{
				ytv1.UpdateStateNone,
				ytv1.UpdateStateWaitingForImageHeater,
			},
		}),
	)
})

var _ = Describe("HasComponent", func() {
	type testCase struct {
		name               string
		updatingComponents []ytv1.Component
		componentType      consts.ComponentType
		expected           bool
	}

	DescribeTable("should correctly check component presence",
		func(tc testCase) {
			result := hasComponent(tc.updatingComponents, tc.componentType)
			Expect(result).To(Equal(tc.expected))
		},
		Entry("nil updating components", testCase{
			name:               "nil updating components",
			updatingComponents: nil,
			componentType:      consts.MasterType,
			expected:           false,
		}),
		Entry("empty updating components", testCase{
			name:               "empty updating components",
			updatingComponents: []ytv1.Component{},
			componentType:      consts.MasterType,
			expected:           false,
		}),
		Entry("component present in updating components", testCase{
			name: "component present in updating components",
			updatingComponents: []ytv1.Component{
				{Type: consts.MasterType},
			},
			componentType: consts.MasterType,
			expected:      true,
		}),
		Entry("component not present in updating components", testCase{
			name: "component not present in updating components",
			updatingComponents: []ytv1.Component{
				{Type: consts.TabletNodeType},
			},
			componentType: consts.MasterType,
			expected:      false,
		}),
	)
})
