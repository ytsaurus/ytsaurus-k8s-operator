package components

import (
	"context"
	"strings"

	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
)

type tabletNode struct {
	server      *Server
	initBundles *InitJob
	master      Component
	labeller    *labeller.Labeller
	cfgen       *ytconfig.Generator
}

func NewTabletNode(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy, master Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: consts.YTComponentLabelTabletNode,
		ComponentName:  "TabletNode",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.TabletNodes.InstanceGroup,
		"/usr/bin/ytserver-node",
		"ytserver-tablet-node.yson",
		"tnd",
		"tablet-nodes",
		false,
		cfgen.GetTabletNodeConfig,
	)

	return &tabletNode{
		server: server,
		initBundles: NewInitJob(
			&labeller,
			apiProxy,
			"bundles",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig),
		master:   master,
		labeller: &labeller,
		cfgen:    cfgen,
	}
}

func (r *tabletNode) createInitScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		"if [[ `/usr/bin/yt exists //sys/tablet_cell_bundles/sys` == 'false' ]]; then /usr/bin/yt create tablet_cell_bundle --attributes '{name=sys; options={changelog_account=sys; snapshot_account=sys}}'; fi",
		"if [[ `/usr/bin/yt get //sys/tablet_cell_bundles/default/@tablet_cell_count --format dsv` -eq 0 ]]; then /usr/bin/yt create tablet_cell --attributes '{tablet_cell_bundle=default}'; fi\n",
		"if [[ `/usr/bin/yt get //sys/tablet_cell_bundles/sys/@tablet_cell_count --format dsv` -eq 0 ]]; then /usr/bin/yt create tablet_cell --attributes '{tablet_cell_bundle=sys}'; fi\n",
	}

	return strings.Join(script, "\n")
}

func (r *tabletNode) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if r.master.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	if !r.server.IsInSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = r.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !r.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	if !dry {
		r.initBundles.SetInitScript(r.createInitScript())
	}

	return r.initBundles.Sync(ctx, dry)
}

func (r *tabletNode) Status(ctx context.Context) SyncStatus {
	status, err := r.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (r *tabletNode) Sync(ctx context.Context) error {
	_, err := r.doSync(ctx, false)
	return err
}

func (r *tabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		r.server,
		r.initBundles,
	})
}
