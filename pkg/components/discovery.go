package components

import (
	"context"
	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
)

type discovery struct {
	apiProxy *apiproxy.ApiProxy

	labeller *labeller.Labeller
	server   *Server
}

func NewDiscovery(cfgen *ytconfig.Generator, apiProxy *apiproxy.ApiProxy) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		ApiProxy:       apiProxy,
		ComponentLabel: "yt-discovery",
		ComponentName:  "Discovery",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.Discovery.InstanceGroup,
		"/usr/bin/ytserver-discovery",
		"ytserver-discovery.yson",
		cfgen.GetDiscoveryStatefulSetName(),
		cfgen.GetDiscoveryServiceName(),
		false,
		cfgen.GetDiscoveryConfig,
	)

	return &discovery{
		server:   server,
		apiProxy: apiProxy,
		labeller: &labeller,
	}
}

func (d *discovery) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		d.server,
	})
}

func (d *discovery) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if !d.server.IsInSync() {
		if !dry {
			err = d.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !d.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	return SyncStatusReady, err
}

func (d *discovery) Status(ctx context.Context) SyncStatus {
	status, err := d.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (d *discovery) Sync(ctx context.Context) error {
	_, err := d.doSync(ctx, false)
	return err
}
