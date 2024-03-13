package ytflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type executionSpy struct {
	recordedEvents []string
}

func (s *executionSpy) record(event string) {
	s.recordedEvents = append(s.recordedEvents, event)
}
func (s *executionSpy) reset() {
	s.recordedEvents = []string{}
}

var (
	dnda = fmt.Sprintf("%sA", DataNodeName)
	dndb = fmt.Sprintf("%sB", DataNodeName)
)

func buildTestComponents(spy *executionSpy) *componentRegistry {
	return &componentRegistry{
		single: map[ComponentName]component{
			YtsaurusClientName: newFakeYtsaurusClient(spy),

			MasterName:    newFakeComponent(MasterName, spy),
			DiscoveryName: newFakeComponent(DiscoveryName, spy),
		},
		multi: map[ComponentName]map[string]component{
			DataNodeName: {
				dnda: newFakeComponent(ComponentName(dnda), spy),
				dndb: newFakeComponent(ComponentName(dndb), spy),
			},
		},
	}
}

func setComponentStatus(comp component, status components.SyncStatus) {
	comp.(*fakeComponent).status = status
}

func buildTestActionSteps(spy *executionSpy) map[StepName]stepType {
	stepNames := []StepName{
		CheckFullUpdatePossibilityStep,
		EnableSafeModeStep,
		BackupTabletCellsStep,
		BuildMasterSnapshotsStep,
		RecoverTabletCellsStep,
		DisableSafeModeStep,
	}
	result := make(map[StepName]stepType)
	for _, name := range stepNames {
		result[name] = newFakeActionStep(name, spy)
	}
	return result
}

func loopAdvance(comps *componentRegistry, actions map[StepName]stepType, state stateManager) error {
	fmt.Println(">>> doAdvance loop")
	defer fmt.Printf("<<< doAdvance end\n\n")

	maxLoops := 3
	for idx := 0; idx < maxLoops; idx++ {
		fmt.Printf("=== LOOP %d\n", idx)
		status, err := doAdvance(context.Background(), comps, actions, state)
		if err != nil {
			return fmt.Errorf("doAdvance failed: %w", err)
		}
		if status == FlowStatusDone {
			return nil
		}
	}
	return fmt.Errorf("advance haven't finished in %d loops", maxLoops)
}

// TestFlows is a series of tests, which share conditions state between them,
// as operator does the same, and we want to check the flow, not internal state correctness.
func TestFlows(t *testing.T) {
	ctx := logr.NewContext(
		context.Background(),
		testr.New(t),
	)
	spy := &executionSpy{}
	comps := buildTestComponents(spy)
	actions := buildTestActionSteps(spy)
	state := newFakeStateManager()
	_ = state.SetClusterState(ctx, ytv1.ClusterStateCreated)

	{
		require.NoError(t, loopAdvance(comps, actions, state))
		// Expect all components created.
		require.Equal(
			t,
			[]string{
				dnda,
				dndb,
				string(DiscoveryName),
				string(MasterName),
				string(YtsaurusClientName),
			},
			spy.recordedEvents,
		)
	}

	{
		spy.reset()
		setComponentStatus(comps.single[DiscoveryName], components.SyncStatusNeedLocalUpdate)

		require.NoError(t, loopAdvance(comps, actions, state))

		// Expect only Discovery updated.
		require.Equal(
			t,
			[]string{
				string(DiscoveryName),
			},
			spy.recordedEvents,
		)
	}

	//{
	//	spy.reset()
	//	setComponentStatus(comps.single[MasterName], components.SyncStatusNeedLocalUpdate)
	//
	//	require.NoError(t, loopAdvance(comps, actions, state))
	//
	//	// Expect full update.
	//	require.Equal(
	//		t,
	//		[]string{
	//			"CheckFullUpdatePossibilityStep",
	//			"EnableSafeModeStep",
	//			"BackupTabletCellsStep",
	//			"BuildMasterSnapshotsStep",
	//			string(MasterName),
	//			"MasterExitReadOnlyStep",
	//			"RecoverTabletCellsStep",
	//			"UpdateOpArchiveStep",
	//			"InitQueryTrackerStep",
	//			"DisableSafeModeStep",
	//		},
	//		spy.recordedEvents,
	//	)
	//}

}
