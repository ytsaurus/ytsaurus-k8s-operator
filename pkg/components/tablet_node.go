package components

import (
	"context"
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type tabletNode struct {
	ComponentBase
	server *Server

	ytsaurusClient YtsaurusClient

	initBundlesCondition string
}

func NewTabletNode(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy, yc YtsaurusClient) Component {
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
		cfgen.GetTabletNodeConfig,
	)

	return &tabletNode{
		ComponentBase: ComponentBase{
			labeller: &labeller,
			apiProxy: apiProxy,
			cfgen:    cfgen,
		},
		server:               server,
		initBundlesCondition: fmt.Sprintf("%s%sInitCompleted", "bundles", labeller.ComponentName),
		ytsaurusClient:       yc,
	}
}

func (r *tabletNode) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	logger := log.FromContext(ctx)

	var err error
	if r.ytsaurusClient.Status(ctx) != SyncStatusReady {
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

	if r.apiProxy.IsStatusConditionTrue(r.initBundlesCondition) {
		return SyncStatusReady, err
	}

	ytClient := r.ytsaurusClient.ytClient

	if !dry {
		if exists, err := ytClient.NodeExists(ctx, ypath.Path("//sys/tablet_cell_bundles/sys"), nil); err == nil {
			if !exists {
				_, err = ytClient.CreateObject(ctx, yt.NodeTabletCellBundle, &yt.CreateObjectOptions{
					Attributes: map[string]interface{}{
						"name": "sys",
						"options": map[string]string{
							"changelog_account": "sys",
							"snapshot_account":  "sys",
						},
					},
				})

				if err != nil {
					logger.Info(fmt.Sprintf("Creating tablet_cell_bundle failed: %s", err.Error()))
					return SyncStatusPending, err
				}
			}
		} else {

			logger.Info(fmt.Sprintf("ERR == nil: %v, %s", (err == nil), err))
			message := err.Error()
			logger.Info(fmt.Sprintf("Checking if exists failed: %s", message))
			return SyncStatusPending, err
		}

		for _, bundle := range []string{"default", "sys"} {
			var tabletCellCount int
			if err := ytClient.GetNode(
				ctx,
				ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", bundle)),
				&tabletCellCount,
				nil); err == nil {
				if tabletCellCount == 0 {
					_, err = ytClient.CreateObject(ctx, "tablet_cell", &yt.CreateObjectOptions{
						Attributes: map[string]interface{}{
							"tablet_cell_bundle": bundle,
						},
					})

					if err != nil {
						logger.Info(fmt.Sprintf("Creating tablet_cell failed: %s", err.Error()))
						return SyncStatusPending, err
					}
				}
			} else {
				logger.Info(fmt.Sprintf("Getting table_cell_count failed: %s", err.Error()))
				return SyncStatusPending, err
			}
		}

		err = r.apiProxy.SetStatusCondition(ctx, metav1.Condition{
			Type:    r.initBundlesCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitBundlesCompleted",
			Message: "Init bundles successfully completed",
		})
	}

	return SyncStatusPending, err
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
	})
}
