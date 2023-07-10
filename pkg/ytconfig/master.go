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

type HydraManager struct {
	MaxChangelogCountToKeep int `yson:"max_changelog_count_to_keep"`
	MaxSnapshotCountToKeep  int `yson:"max_snapshot_count_to_keep"`
}

type CypressManager struct {
	DefaultTableReplicationFactor   int `yson:"default_table_replication_factor,omitempty"`
	DefaultFileReplicationFactor    int `yson:"default_file_replication_factor,omitempty"`
	DefaultJournalReplicationFactor int `yson:"default_journal_replication_factor,omitempty"`
	DefaultJournalReadQuorum        int `yson:"default_journal_read_quorum,omitempty"`
	DefaultJournalWriteQuorum       int `yson:"default_journal_write_quorum,omitempty"`
}

type MasterServer struct {
	CommonServer
	Snapshots        MasterSnapshots  `yson:"snapshots"`
	Changelogs       MasterChangelogs `yson:"changelogs"`
	UseNewHydra      bool             `yson:"use_new_hydra"`
	HydraManager     HydraManager     `yson:"hydra_manager"`
	CypressManager   CypressManager   `yson:"cypress_manager"`
	PrimaryMaster    MasterCell       `yson:"primary_master"`
	SecondaryMasters []MasterCell     `yson:"secondary_masters"`
}

func getMasterServerCarcass(spec ytv1.MastersSpec) (MasterServer, error) {
	var c MasterServer
	c.UseNewHydra = true
	c.RPCPort = consts.MasterRPCPort
	c.MonitoringPort = consts.MasterMonitoringPort
	c.HydraManager.MaxSnapshotCountToKeep = 2
	c.HydraManager.MaxChangelogCountToKeep = 2
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

	c.Logging = createLogging(&spec.InstanceSpec, "master", []ytv1.LoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})

	return c, nil
}
