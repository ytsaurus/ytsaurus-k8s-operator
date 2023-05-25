package components

import (
	"context"
	"fmt"
	"strings"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type queryTracker struct {
	ServerComponentBase
	master          Component
	initEnvironment *InitJob
}

func NewQueryTracker(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy, master Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-query-tracker",
		ComponentName:  "QueryTracker",
		MonitoringPort: consts.QueryTrackerMonitoringPort,
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.QueryTrackers.InstanceSpec,
		"/usr/bin/ytserver-query-tracker",
		"ytserver-query-tracker.yson",
		cfgen.GetQueryTrackerStatefulSetName(),
		cfgen.GetQueryTrackerServiceName(),
		cfgen.GetQueryTrackerConfig,
	)

	return &queryTracker{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &labeller,
				apiProxy: apiProxy,
				cfgen:    cfgen,
			},
			server: server,
		},
		master: master,
		initEnvironment: NewInitJob(
			&labeller,
			apiProxy,
			"qt-environment",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig),
	}
}

func (qt *queryTracker) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		qt.server,
		qt.initEnvironment,
	})
}

func (qt *queryTracker) createInitScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		"/usr/bin/yt create user --attributes '{name=query_tracker}' --ignore-existing",
		"/usr/bin/yt add-member --member query_tracker --group superusers || true",
		fmt.Sprintf("/usr/bin/yt create document //sys/query_tracker/config --attributes '{value={query_tracker={ql_engine={default_cluster=\"%s\"}; chyt_engine={default_cluster=\"%s\"}}}}' --recursive --ignore-existing", qt.labeller.GetClusterName(), qt.labeller.GetClusterName()),
		"/usr/bin/yt set //sys/@cluster_connection/query_tracker '{stages={production={root=\"//sys/query_tracker\"; user=query_tracker}}}'",
		fmt.Sprintf("/usr/bin/yt get //sys/@cluster_connection | /usr/bin/yt set //sys/clusters/%s", qt.labeller.GetClusterName()),
	}

	return strings.Join(script, "\n")
}

func (qt *queryTracker) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error

	if qt.apiProxy.GetClusterState() == ytv1.ClusterStateUpdating {
		if qt.apiProxy.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			return SyncStatusUpdating, qt.removePods(ctx, dry)
		}
	}

	if qt.master.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	if !qt.server.IsInSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = qt.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !qt.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	if !dry {
		qt.initEnvironment.SetInitScript(qt.createInitScript())
	}

	return qt.initEnvironment.Sync(ctx, dry)
}

func (qt *queryTracker) Status(ctx context.Context) SyncStatus {
	status, err := qt.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (qt *queryTracker) Sync(ctx context.Context) error {
	_, err := qt.doSync(ctx, false)
	return err
}
