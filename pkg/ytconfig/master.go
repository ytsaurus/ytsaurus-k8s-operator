package ytconfig

import (
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type MasterChangelogs struct {
	Path string `yson:"path"`
}

type MasterSnapshots struct {
	Path string `yson:"path"`
}

type Hydra struct {
	MaxChangelogCountToKeep int `yson:"max_changelog_count_to_keep"`
	MaxSnapshotCountToKeep  int `yson:"max_snapshot_count_to_keep"`
}

type MasterServer struct {
	CommonServer
	Snapshots        MasterSnapshots  `yson:"snapshots"`
	Changelogs       MasterChangelogs `yson:"changelogs"`
	UseNewHydra      bool             `yson:"use_new_hydra"`
	Hydra            Hydra            `yson:"hydra"`
	PrimaryMaster    MasterCell       `yson:"primary_master"`
	SecondaryMasters []MasterCell     `yson:"secondary_masters"`
}

func getMasterServerCarcass(spec ytv1.MastersSpec) (MasterServer, error) {
	var c MasterServer
	c.UseNewHydra = true
	c.RPCPort = consts.MasterRPCPort
	c.MonitoringPort = consts.MasterMonitoringPort
	c.Hydra.MaxSnapshotCountToKeep = 2
	c.Hydra.MaxChangelogCountToKeep = 2
	c.SecondaryMasters = make([]MasterCell, 0)

	if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeMasterChangelogs); location != nil {
		c.Changelogs.Path = location.Path
	} else {
		return c, fmt.Errorf("error creating master config: changelog location not found")
	}

	if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeMasterSnapshots); location != nil {
		c.Snapshots.Path = location.Path
	} else {
		return c, fmt.Errorf("error creating master config: snapshot location not found")
	}

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeLogs), "master")
	c.Logging = loggingBuilder.addDefaultInfo().addDefaultStderr().logging

	return c, nil
}
