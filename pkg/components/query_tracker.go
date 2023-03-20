package components

import (
	"context"
	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
	"strings"
)

type queryTracker struct {
	server          *Server
	master          Component
	initEnvironment *InitJob
	labeller        *labeller.Labeller
}

func NewQueryTracker(cfgen *ytconfig.Generator, apiProxy *apiproxy.ApiProxy, master Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		ApiProxy:       apiProxy,
		ComponentLabel: "yt-query-tracker",
		ComponentName:  "QueryTracker",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.QueryTrackers.InstanceGroup,
		"/usr/bin/ytserver-query-tracker",
		"ytserver-query-tracker.yson",
		cfgen.GetQueryTrackerStatefulSetName(),
		cfgen.GetQueryTrackerServiceName(),
		false,
		cfgen.GetQueryTrackerConfig,
	)

	return &queryTracker{
		server: server,
		master: master,
		initEnvironment: NewInitJob(
			&labeller,
			apiProxy,
			"qt-environment",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig),
		labeller: &labeller,
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
		"/usr/bin/yt create document //sys/query_tracker/config --attributes '{value={query_tracker={ql_engine={default_cluster=yt}; chyt_engine={default_cluster=yt}}}}' --recursive --ignore-existing",
		"/usr/bin/yt set //sys/@cluster_connection/query_tracker '{stages={production={root=\"//sys/query_tracker\"; user=query_tracker}}}'",
		"/usr/bin/yt get //sys/@cluster_connection | /usr/bin/yt set //sys/clusters/yt",
	}

	return strings.Join(script, "\n")
}

func (qt *queryTracker) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
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
