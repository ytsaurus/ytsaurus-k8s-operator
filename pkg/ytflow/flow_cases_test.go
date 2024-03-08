package ytflow

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type executionSpy struct {
	recordedEvents []string
}

func (s *executionSpy) record(event string) {
	s.recordedEvents = append(s.recordedEvents, event)
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

func TestNormalFlow(t *testing.T) {
	ctx := context.Background()
	spy := &executionSpy{}
	comps := buildTestComponents(spy)
	conds := newFakeConditionManager()

	for loops := 0; loops < 3; loops++ {
		status, err := doAdvance(ctx, comps, conds)
		require.NoError(t, err)
		if status == FlowStatusDone {
			break
		}
	}

	// initializing
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

	spy.recordedEvents = []string{}
	comps.single[DiscoveryName].(*fakeComponent).ran = false
	for loops := 0; loops < 3; loops++ {
		status, err := doAdvance(ctx, comps, conds)
		require.NoError(t, err)
		if status == FlowStatusDone {
			break
		}
	}

	// update only discovery
	require.Equal(
		t,
		[]string{
			string(DiscoveryName),
		},
		spy.recordedEvents,
	)

}
