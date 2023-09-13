package components

import (
	"context"
	"fmt"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
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
	serverComponentBase

	ytsaurusClient YtsaurusClient

	initBundlesCondition string
	spec                 ytv1.TabletNodesSpec
	doInitialization     bool
}

func NewTabletNode(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	ytsaurusClient YtsaurusClient,
	spec ytv1.TabletNodesSpec,
	doInitiailization bool,
) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelTabletNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("TabletNode", spec.Name),
		MonitoringPort: consts.NodeMonitoringPort,
	}

	server := newServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-tablet-node.yson",
		cfgen.GetTabletNodesStatefulSetName(spec.Name),
		cfgen.GetTabletNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetTabletNodeConfig(spec)
		},
	)

	return &tabletNode{
		serverComponentBase: serverComponentBase{
			componentBase: componentBase{
				labeller: &l,
				ytsaurus: ytsaurus,
				cfgen:    cfgen,
			},
			server: server,
		},
		initBundlesCondition: "bundlesTabletNodeInitCompleted",
		ytsaurusClient:       ytsaurusClient,
		spec:                 spec,
		doInitialization:     doInitiailization,
	}
}

func (tn *tabletNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	logger := log.FromContext(ctx)

	if tn.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && tn.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), err
	}

	if tn.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating && tn.IsUpdating() {
		if tn.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			if !dry {
				err = tn.removePods(ctx)
			}
			return WaitingStatus(SyncStatusUpdating, "pods removal"), err
		}
	}

	if tn.server.needSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = tn.server.Sync(ctx)
		}

		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !tn.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	if !tn.doInitialization || tn.ytsaurus.IsStatusConditionTrue(tn.initBundlesCondition) {
		return SimpleStatus(SyncStatusReady), err
	}

	if tn.ytsaurusClient.Status(ctx).SyncStatus != SyncStatusReady {
		return WaitingStatus(SyncStatusBlocked, tn.ytsaurusClient.GetName()), err
	}

	ytClient := tn.ytsaurusClient.GetYtClient()

	if !dry {
		// TODO: refactor it
		if tn.doInitialization {
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
						logger.Error(err, "Creating tablet_cell_bundle failed")
						return WaitingStatus(SyncStatusPending, "tablet_cell_bundle creation"), err
					}
				}
			} else {
				return WaitingStatus(SyncStatusPending, "tablet_cell_bundle creation"), err
			}

			for _, bundle := range []string{"default", "sys"} {
				err = CreateTabletCells(ctx, ytClient, bundle, 1)
				if err != nil {
					return WaitingStatus(SyncStatusPending, "tablet cells creation"), err
				}
			}

			tn.ytsaurus.SetStatusCondition(metav1.Condition{
				Type:    tn.initBundlesCondition,
				Status:  metav1.ConditionTrue,
				Reason:  "InitBundlesCompleted",
				Message: "Init bundles successfully completed",
			})
		}
	}

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", tn.initBundlesCondition)), err
}

func (tn *tabletNode) Status(ctx context.Context) ComponentStatus {
	status, err := tn.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (tn *tabletNode) Sync(ctx context.Context) error {
	_, err := tn.doSync(ctx, false)
	return err
}

func (tn *tabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		tn.server,
	})
}
