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
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type queryTracker struct {
	ServerComponentBase

	ytsaurusClient YtsaurusClient
	initCondition  string
}

func NewQueryTracker(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	yc YtsaurusClient,
) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: "yt-query-tracker",
		ComponentName:  "QueryTracker",
		MonitoringPort: consts.QueryTrackerMonitoringPort,
	}

	server := NewServer(
		&l,
		ytsaurus,
		&resource.Spec.QueryTrackers.InstanceSpec,
		"/usr/bin/ytserver-query-tracker",
		"ytserver-query-tracker.yson",
		cfgen.GetQueryTrackerStatefulSetName(),
		cfgen.GetQueryTrackerServiceName(),
		cfgen.GetQueryTrackerConfig,
		cfgen.NeedQueryTrackerConfigReload,
	)

	return &queryTracker{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &l,
				ytsaurus: ytsaurus,
				cfgen:    cfgen,
			},
			server: server,
		},
		initCondition:  "queryTrackerInitCompleted",
		ytsaurusClient: yc,
	}
}

func (qt *queryTracker) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		qt.server,
	})
}

func (qt *queryTracker) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error

	if qt.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && qt.server.NeedUpdate() {
		return SyncStatusNeedLocalUpdate, err
	}

	if qt.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if qt.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			updatingComponents := qt.ytsaurus.GetLocalUpdatingComponents()
			if updatingComponents == nil || slices.Contains(updatingComponents, qt.GetName()) {
				return SyncStatusUpdating, qt.removePods(ctx, dry)
			}
		}
	}

	if qt.server.NeedSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = qt.server.Sync(ctx)
		}

		return SyncStatusPending, err
	}

	if !qt.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	if qt.ytsaurus.IsStatusConditionTrue(qt.initCondition) {
		return SyncStatusReady, err
	}

	if qt.ytsaurusClient.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	ytClient := qt.ytsaurusClient.GetYtClient()

	if !dry {
		err = qt.init(ctx, ytClient)
		if err != nil {
			return SyncStatusPending, err
		}

		err = qt.ytsaurus.SetStatusCondition(ctx, metav1.Condition{
			Type:    qt.initCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitQueryTrackerCompleted",
			Message: "Init query tracker successfully completed",
		})
	}

	return SyncStatusPending, err
}

func (qt *queryTracker) init(ctx context.Context, ytClient yt.Client) (err error) {
	logger := log.FromContext(ctx)

	_, err = ytClient.CreateObject(
		ctx,
		yt.NodeUser,
		&yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"name": "query_tracker",
			},
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating user 'query_tracker' failed")
		return
	}

	_ = ytClient.AddMember(ctx, "superusers", "query_tracker", nil)

	_, err = ytClient.CreateNode(
		ctx,
		ypath.Path("//sys/query_tracker/config"),
		yt.NodeDocument,
		&yt.CreateNodeOptions{
			Attributes: map[string]interface{}{
				"value": map[string]interface{}{
					"query_tracker": map[string]interface{}{
						"ql_engine": map[string]interface{}{
							"default_cluster": qt.labeller.GetClusterName(),
						},
						"chyt_engine": map[string]interface{}{
							"default_cluster": qt.labeller.GetClusterName(),
						},
					},
				},
			},
			Recursive:      true,
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating document '//sys/query_tracker/config' failed")
		return
	}

	err = ytClient.SetNode(
		ctx,
		ypath.Path("//sys/@cluster_connection/query_tracker"),
		map[string]interface{}{
			"stages": map[string]interface{}{
				"production": map[string]interface{}{
					"root": "//sys/query_tracker",
					"user": "query_tracker",
				},
			},
		},
		nil,
	)
	if err != nil {
		logger.Error(err, "Setting '//sys/@cluster_connection/query_tracker' failed")
		return
	}

	clusterConnectionAttr := make(map[string]interface{})
	err = ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection"), &clusterConnectionAttr, nil)
	if err != nil {
		logger.Error(err, "Getting '//sys/@cluster_connection' failed")
		return
	}

	err = ytClient.SetNode(
		ctx,
		ypath.Path(fmt.Sprintf("//sys/clusters/%s", qt.labeller.GetClusterName())),
		clusterConnectionAttr,
		nil,
	)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Setting '//sys/clusters/%s' failed", qt.labeller.GetClusterName()))
		return
	}
	return
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
