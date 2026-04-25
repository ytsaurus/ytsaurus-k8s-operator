package checks

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// NYT::NHydra::EPeerState
type MasterState string

const (
	MasterStateLeading   MasterState = "leading"
	MasterStateFollowing MasterState = "following"
)

// See: https://github.com/ytsaurus/ytsaurus/blob/main/yt/yt/server/lib/hydra/distributed_hydra_manager.cpp
type MasterHydra struct {
	State                MasterState `yson:"state"`
	SelfID               int32       `yson:"self_id"`
	LeaderID             int32       `yson:"leader_id"`
	Voting               bool        `yson:"voting"`
	Active               bool        `yson:"active"`
	ActiveLeader         bool        `yson:"active_leader"`
	ActiveFollower       bool        `yson:"active_follower"`
	BuildingSnapshot     bool        `yson:"building_snapshot"`
	EnteringReadOnlyMode bool        `yson:"entering_read_only_mode"`
	LastSnapshotReadOnly bool        `yson:"last_snapshot_read_only"`
	ReadOnly             bool        `yson:"read_only"`
}

type MasterAddressWithAttributes struct {
	Address     string `yson:",value"`
	Maintenance bool   `yson:"maintenance,attr"`
}

func MasterQuorumHealth(
	ctx context.Context,
	ytClient yt.Client,
	mastersPath ypath.Path,
	totalCount int32,
	requiredCount int32,
) (ok bool, message string, err error) {
	masters := make([]MasterAddressWithAttributes, 0, totalCount)
	err = ytClient.ListNode(ctx, mastersPath, &masters, &yt.ListNodeOptions{
		Attributes: []string{"maintenance"},
	})
	if err != nil {
		return false, "", err
	}

	leaders := make([]string, 0, 1)
	followers := make([]string, 0, totalCount)
	var inactive, maintenance []string
	for _, master := range masters {
		var hydra MasterHydra
		hydraPath := mastersPath.Child(master.Address).Child(consts.MasterHydraPath)
		if err := ytClient.GetNode(ctx, hydraPath, &hydra, nil); err != nil {
			return false, "", err
		}
		name := fmt.Sprintf("[%d] %s", hydra.SelfID, master.Address)
		if !hydra.Active {
			inactive = append(inactive, name)
		}
		if master.Maintenance {
			maintenance = append(maintenance, name)
		}
		switch hydra.State {
		case MasterStateLeading:
			leaders = append(leaders, name)
		case MasterStateFollowing:
			followers = append(followers, name)
		}
	}

	var note string
	switch {
	case len(maintenance) != 0:
		note = "There is a master in maintenance"
	case len(inactive) != 0:
		note = "There is a non-active master"
	case len(leaders) != 1:
		note = "There is no single leader"
	case len(followers)+1 < int(requiredCount):
		note = "Not enough followers"
	default:
		note = "Quorum is OK"
		ok = true
	}

	message = fmt.Sprintf("%v: leaders/followers/required/total=%d/%d/%d/%d, leaders=%v, followers=%v, inactive=%v, maintenance=%v",
		note, len(leaders), len(followers), requiredCount, totalCount, leaders, followers, inactive, maintenance)
	return ok, message, nil
}
