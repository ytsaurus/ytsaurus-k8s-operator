package ytconfig

import (
	"fmt"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type MasterChangelogs struct {
	Path     string    `yson:"path"`
	IOEngine *IOEngine `yson:"io_engine,omitempty"`
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

func getMasterLogging(spec *ytv1.MastersSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"master",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getMasterServerCarcass(spec *ytv1.MastersSpec) (MasterServer, error) {
	var c MasterServer
	c.UseNewHydra = true
	c.RPCPort = consts.MasterRPCPort
	if spec.MonitoringPort != nil {
		c.MonitoringPort = *spec.MonitoringPort
	}

	c.HydraManager.MaxSnapshotCountToKeep = 10
	if spec.MaxSnapshotCountToKeep != nil {
		c.HydraManager.MaxSnapshotCountToKeep = *spec.MaxSnapshotCountToKeep
	}
	c.HydraManager.MaxChangelogCountToKeep = 10
	if spec.MaxChangelogCountToKeep != nil {
		c.HydraManager.MaxChangelogCountToKeep = *spec.MaxChangelogCountToKeep
	}

	c.SecondaryMasters = nil

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

	c.Logging = getMasterLogging(spec)

	return c, nil
}

func configureMasterServerCypressManager(maxReplicationFactor int32, c *CypressManager) {
	switch {
	case maxReplicationFactor <= 1:
		c.DefaultJournalReadQuorum = 1
		c.DefaultJournalWriteQuorum = 1
		c.DefaultTableReplicationFactor = 1
		c.DefaultFileReplicationFactor = 1
		c.DefaultJournalReplicationFactor = 1
	case maxReplicationFactor == 2:
		c.DefaultJournalReadQuorum = 2
		c.DefaultJournalWriteQuorum = 2
		c.DefaultTableReplicationFactor = 2
		c.DefaultFileReplicationFactor = 2
		c.DefaultJournalReplicationFactor = 2
	case maxReplicationFactor >= 3 && maxReplicationFactor < 5:
		c.DefaultJournalReadQuorum = 2
		c.DefaultJournalWriteQuorum = 2
		c.DefaultTableReplicationFactor = 3
		c.DefaultFileReplicationFactor = 3
		c.DefaultJournalReplicationFactor = 3
	case maxReplicationFactor >= 5:
		c.DefaultJournalReadQuorum = 3
		c.DefaultJournalWriteQuorum = 3
		c.DefaultTableReplicationFactor = 3
		c.DefaultFileReplicationFactor = 3
		c.DefaultJournalReplicationFactor = 5
	}
}
