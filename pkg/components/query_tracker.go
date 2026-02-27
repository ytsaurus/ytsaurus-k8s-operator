package components

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type QueryTracker struct {
	serverComponent

	cfgen *ytconfig.Generator

	ytsaurusClient internalYtsaurusClient
	tabletNodes    []Component
	initCondition  string
	initQTState    *InitJob
	secret         *resources.StringSecret
}

func NewQueryTracker(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	yc internalYtsaurusClient,
	tabletNodes []Component,
) *QueryTracker {
	l := cfgen.GetComponentLabeller(consts.QueryTrackerType, "")
	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.QueryTrackers.InstanceSpec,
		"/usr/bin/ytserver-query-tracker",
		[]ConfigGenerator{{
			"ytserver-query-tracker.yson",
			ConfigFormatYson,
			func() ([]byte, error) { return cfgen.GetQueryTrackerConfig(resource.Spec.QueryTrackers) },
		}},
		consts.QueryTrackerMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.QueryTrackerRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &QueryTracker{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
		tabletNodes:     tabletNodes,
		initCondition:   "queryTrackerInitCompleted",
		ytsaurusClient:  yc,
		initQTState: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"qt-state",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&resource.Spec.QueryTrackers.InstanceSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus),
	}
}

func (qt *QueryTracker) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		qt.server,
		qt.initQTState,
		qt.secret,
	)
}

func (qt *QueryTracker) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if qt.ytsaurus.IsReadyToUpdate() && qt.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if qt.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(qt.ytsaurus, qt) {
			// Handle bulk update with pre-checks
			if status, err := handleBulkUpdatingClusterState(ctx, qt.ytsaurus, qt, &qt.component, qt.server, dry); status != nil {
				return *status, err
			}

			if status, err := qt.updateQTState(ctx, dry); status != nil {
				return *status, err
			}
			if qt.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation &&
				qt.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForQTStateUpdate {
				return ComponentStatusReady(), err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), err
		}
	}

	if status, err := syncUserToken(ctx, qt.ytsaurusClient, qt.secret, consts.QueryTrackerUserName, consts.SuperusersGroupName, dry); !status.IsRunning() {
		return status, err
	}

	if qt.NeedSync() {
		if !dry {
			err = qt.server.Sync(ctx)
		}

		return ComponentStatusWaitingFor("components"), err
	}

	if !qt.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	// Wait for tablet nodes to proceed with query tracker state init.
	if len(qt.tabletNodes) == 0 {
		return ComponentStatusBlockedBy("tablet nodes"), fmt.Errorf("cannot initialize query tracker without tablet nodes")
	}

	for _, tnd := range qt.tabletNodes {
		tndStatus, err := tnd.Status(ctx)
		if err != nil {
			return tndStatus, err
		}
		if !tndStatus.IsRunning() {
			return ComponentStatusBlockedBy("tablet nodes"), err
		}
	}

	var ytClient yt.Client
	if !dry {
		ytClient = qt.ytsaurusClient.GetYtClient()
		if ytClient == nil {
			return ComponentStatusWaitingFor("getting yt client"), err
		}
	}

	if !dry {
		err = qt.init(ctx, ytClient)
		if err != nil {
			return ComponentStatusWaitingFor(fmt.Sprintf("%s initialization", qt.GetFullName())), err
		}

		qt.ytsaurus.SetStatusCondition(metav1.Condition{
			Type:    qt.initCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitQueryTrackerCompleted",
			Message: "Init query tracker successfully completed",
		})
	}

	if !dry {
		qt.prepareInitQueryTrackerState()
	}
	status, err := qt.initQTState.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if qt.ytsaurus.IsStatusConditionTrue(qt.initCondition) {
		return ComponentStatusReady(), err
	}
	return ComponentStatusWaitingFor(fmt.Sprintf("setting %s condition", qt.initCondition)), err
}

func (qt *QueryTracker) init(ctx context.Context, ytClient yt.Client) (err error) {
	logger := log.FromContext(ctx)

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
						"spyt_engine": map[string]interface{}{
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
		return err
	}

	err = ytClient.SetNode(
		ctx,
		ypath.Path("//sys/@cluster_connection/query_tracker"),
		map[string]interface{}{
			"stages": map[string]interface{}{
				"production": map[string]interface{}{
					"root":    "//sys/query_tracker",
					"user":    "query_tracker",
					"channel": map[string]interface{}{"addresses": qt.cfgen.GetQueryTrackerAddresses()},
				},
			},
		},
		nil,
	)
	if err != nil {
		logger.Error(err, "Setting '//sys/@cluster_connection/query_tracker' failed")
		return err
	}

	clusterConnectionAttr := make(map[string]interface{})
	err = ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection"), &clusterConnectionAttr, nil)
	if err != nil {
		logger.Error(err, "Getting '//sys/@cluster_connection' failed")
		return err
	}

	err = ytClient.SetNode(
		ctx,
		ypath.Path(fmt.Sprintf("//sys/clusters/%s", qt.labeller.GetClusterName())),
		clusterConnectionAttr,
		nil,
	)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Setting '//sys/clusters/%s' failed", qt.labeller.GetClusterName()))
		return err
	}

	_, err = ytClient.CreateObject(
		ctx,
		yt.NodeAccessControlObjectNamespace,
		&yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"name": "queries",
				"acl": []interface{}{
					map[string]interface{}{
						"action":           "allow",
						"subjects":         []string{"owner"},
						"permissions":      []string{"read", "write", "administer", "remove"},
						"inheritance_mode": "immediate_descendants_only",
					},
					map[string]interface{}{
						"action":           "allow",
						"subjects":         []string{"users"},
						"permissions":      []string{"modify_children"},
						"inheritance_mode": "object_only",
					},
				},
			},
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating access control object namespace 'queries' failed")
		return err
	}

	_, err = ytClient.CreateObject(
		ctx,
		yt.NodeAccessControlObject,
		&yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"name":      "nobody",
				"namespace": "queries",
			},
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating access control object 'nobody' in namespace 'queries' failed")
		return err
	}

	_, err = ytClient.CreateObject(
		ctx,
		yt.NodeAccessControlObject,
		&yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"name":      "everyone",
				"namespace": "queries",
				"principal_acl": []interface{}{map[string]interface{}{
					"action":      "allow",
					"subjects":    []string{"everyone"},
					"permissions": []string{"read", "use"},
				}},
			},
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating access control object 'everyone' in namespace 'queries' failed")
		return err
	}

	_, err = ytClient.CreateObject(
		ctx,
		yt.NodeAccessControlObject,
		&yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"name":      "everyone-use",
				"namespace": "queries",
				"principal_acl": []interface{}{map[string]interface{}{
					"action":      "allow",
					"subjects":    []string{"everyone"},
					"permissions": []string{"use"},
				}},
			},
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating access control object 'everyone-use' in namespace 'queries' failed")
		return err
	}

	_, err = ytClient.CreateObject(
		ctx,
		yt.NodeAccessControlObject,
		&yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"name":      "everyone-share",
				"namespace": "queries",
				"principal_acl": []interface{}{map[string]interface{}{
					"action":      "allow",
					"subjects":    []string{"everyone"},
					"permissions": []string{"read", "use"},
				}},
			},
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating access control object 'everyone-share' in namespace 'queries' failed")
		return err
	}
	return nil
}

func (qt *QueryTracker) Status(ctx context.Context) (ComponentStatus, error) {
	return qt.doSync(ctx, true)
}

func (qt *QueryTracker) Sync(ctx context.Context) error {
	_, err := qt.doSync(ctx, false)
	return err
}

func (qt *QueryTracker) prepareInitQueryTrackerState() {
	path := "/usr/bin/init_query_tracker_state"

	script := []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf("if [[ -f \"%s\" ]]; then %s --force --latest --proxy %s; fi",
			path, path, qt.cfgen.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole)),
	}

	qt.initQTState.SetInitScript(strings.Join(script, "\n"))
	job := qt.initQTState.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{qt.secret.GetEnvSource()}
}

func (qt *QueryTracker) updateQTState(ctx context.Context, dry bool) (*ComponentStatus, error) {
	var err error
	switch qt.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForQTStateUpdatingPrepare:
		if !qt.initQTState.isRestartPrepared() {
			return ptr.To(SimpleStatus(SyncStatusUpdating)), qt.initQTState.prepareRestart(ctx, dry)
		}
		if !dry {
			qt.setConditionQTStatePreparedForUpdating(ctx)
		}
		return ptr.To(SimpleStatus(SyncStatusUpdating)), err
	case ytv1.UpdateStateWaitingForQTStateUpdate:
		if !qt.initQTState.isRestartCompleted() {
			return nil, nil
		}
		if !dry {
			qt.setConditionQTStateUpdated(ctx)
		}
		return ptr.To(SimpleStatus(SyncStatusUpdating)), err
	default:
		return nil, nil
	}
}

func (qt *QueryTracker) setConditionQTStatePreparedForUpdating(ctx context.Context) {
	qt.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionQTStatePreparedForUpdating,
		Status:  metav1.ConditionTrue,
		Reason:  "QTStatePreparedForUpdating",
		Message: "Query Tracker state prepared for updating",
	})
}

func (qt *QueryTracker) setConditionQTStateUpdated(ctx context.Context) {
	qt.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionQTStateUpdated,
		Status:  metav1.ConditionTrue,
		Reason:  "QTStateUpdated",
		Message: "Query tracker state updated",
	})
}

func (qt *QueryTracker) UpdatePreCheck(ctx context.Context) ComponentStatus {
	// Get YT client from the ytsaurusClient component
	if qt.ytsaurusClient == nil {
		return ComponentStatusBlocked("YtsaurusClient component is not available")
	}
	ytClient := qt.ytsaurusClient.GetYtClient()

	// Check that the number of instances in YT matches the expected instanceCount
	expectedCount := int(qt.ytsaurus.GetResource().Spec.QueryTrackers.InstanceCount)
	if err := IsInstanceCountEqualYTSpec(ctx, ytClient, consts.QueryTrackerType, expectedCount); err != nil {
		return ComponentStatusBlocked(err.Error())
	}

	return ComponentStatusReady()
}
