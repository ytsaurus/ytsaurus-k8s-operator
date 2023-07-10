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
	Hydra            Hydra            `yson:"hydra"`
	CypressManager   CypressManager   `yson:"cypress_manager"`
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

	c.Logging = createLogging(&spec.InstanceSpec, "master", []ytv1.LoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})

	return c, nil
}

func configureMasterServerCypressManager(spec ytv1.YtsaurusSpec, c *CypressManager) {
	instanceCount := int32(0)
	for _, dataNodes := range spec.DataNodes {
		instanceCount += dataNodes.InstanceCount
	}

	switch {
	case instanceCount <= 1:
		c.DefaultJournalReadQuorum = 1
		c.DefaultJournalWriteQuorum = 1
		c.DefaultTableReplicationFactor = 1
		c.DefaultFileReplicationFactor = 1
		c.DefaultJournalReplicationFactor = 1
	case instanceCount == 2:
		c.DefaultJournalReadQuorum = 2
		c.DefaultJournalWriteQuorum = 2
		c.DefaultTableReplicationFactor = 2
		c.DefaultFileReplicationFactor = 2
		c.DefaultJournalReplicationFactor = 2
	case instanceCount >= 3 && instanceCount < 5:
		c.DefaultJournalReadQuorum = 3
		c.DefaultJournalWriteQuorum = 3
		c.DefaultTableReplicationFactor = 3
		c.DefaultFileReplicationFactor = 3
		c.DefaultJournalReplicationFactor = 3
	case instanceCount >= 5:
		c.DefaultJournalReadQuorum = 3
		c.DefaultJournalWriteQuorum = 3
		c.DefaultTableReplicationFactor = 3
		c.DefaultFileReplicationFactor = 3
		c.DefaultJournalReplicationFactor = 5
	}
}
