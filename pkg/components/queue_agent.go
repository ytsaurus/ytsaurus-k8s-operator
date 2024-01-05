package components

import (
	"context"
	"fmt"
	"strings"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type queueAgent struct {
	ytsaurusServerComponent
	cfgen *ytconfig.Generator

	ytsaurusClient YtsaurusClient
	master         Component
	tabletNodes    []Component
	initCondition  string
	initQAState    *InitJob
	secret         *resources.StringSecret
}

func NewQueueAgent(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	yc YtsaurusClient,
	master Component,
	tabletNodes []Component,
) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: "yt-queue-agent",
		ComponentName:  "QueueAgent",
		MonitoringPort: consts.QueueAgentMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.QueueAgents.InstanceSpec,
		"/usr/bin/ytserver-queue-agent",
		"ytserver-queue-agent.yson",
		cfgen.GetQueueAgentStatefulSetName(),
		cfgen.GetQueueAgentServiceName(),
		cfgen.GetQueueAgentConfig,
	)

	image := ytsaurus.GetResource().Spec.CoreImage
	if resource.Spec.QueueAgents.InstanceSpec.Image != nil {
		image = *resource.Spec.QueueAgents.InstanceSpec.Image
	}

	return &queueAgent{
		ytsaurusServerComponent: newYtsaurusServerComponent(&l, ytsaurus, srv),
		cfgen:                   cfgen,
		master:                  master,
		tabletNodes:             tabletNodes,
		initCondition:           "queueAgentInitCompleted",
		ytsaurusClient:          yc,
		initQAState: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"qa-state",
			consts.ClientConfigFileName,
			image,
			cfgen.GetNativeClientConfig),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
	}
}

func (qa *queueAgent) IsUpdatable() bool {
	return true
}

func (qa *queueAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		qa.server,
		qa.initQAState,
		qa.secret,
	)
}

func (qa *queueAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(qa.ytsaurus.GetClusterState()) && qa.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if qa.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, qa.ytsaurus, qa, &qa.ytsaurusComponent, qa.server, dry); status != nil {
			return *status, err
		}
	}

	if !IsRunningStatus(qa.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, qa.master.GetName()), err
	}

	// It makes no sense to start queue agents without tablet nodes.
	if qa.tabletNodes == nil || len(qa.tabletNodes) == 0 {
		return WaitingStatus(SyncStatusBlocked, "tablet nodes"), fmt.Errorf("cannot initialize queue agent without tablet nodes")
	}
	for _, tnd := range qa.tabletNodes {
		if !IsRunningStatus(tnd.Status(ctx).SyncStatus) {
			return WaitingStatus(SyncStatusBlocked, tnd.GetName()), err
		}
	}

	if qa.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			secretSpec := qa.secret.Build()
			secretSpec.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = qa.secret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, qa.secret.Name()), err
	}

	if qa.NeedSync() {
		if !dry {
			err = qa.server.Sync(ctx)
		}

		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !qa.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	var ytClient yt.Client
	if qa.ytsaurus.GetClusterState() != ytv1.ClusterStateUpdating {
		if qa.ytsaurusClient.Status(ctx).SyncStatus != SyncStatusReady {
			return WaitingStatus(SyncStatusBlocked, qa.ytsaurusClient.GetName()), err
		}

		if !dry {
			ytClient = qa.ytsaurusClient.GetYtClient()

			err = qa.createUser(ctx, ytClient)
			if err != nil {
				return WaitingStatus(SyncStatusPending, "create qa user"), err
			}
		}
	}

	if !dry {
		qa.prepareInitQueueAgentState()
	}
	status, err := qa.initQAState.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if qa.ytsaurus.IsStatusConditionTrue(qa.initCondition) {
		return SimpleStatus(SyncStatusReady), err
	}

	if !dry {
		err = qa.init(ctx, ytClient)
		if err != nil {
			return WaitingStatus(SyncStatusPending, fmt.Sprintf("%s initialization", qa.GetName())), err
		}

		qa.ytsaurus.SetStatusCondition(metav1.Condition{
			Type:    qa.initCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitQueueAgentCompleted",
			Message: "Init queue agent successfully completed",
		})
	}

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", qa.initCondition)), err
}

func (qa *queueAgent) createUser(ctx context.Context, ytClient yt.Client) (err error) {
	logger := log.FromContext(ctx)

	token, _ := qa.secret.GetValue(consts.TokenSecretKey)
	err = CreateUser(ctx, ytClient, "queue_agent", token, true)
	if err != nil {
		logger.Error(err, "Creating user 'queue_agent' failed")
		return
	}
	return
}

func (qa *queueAgent) init(ctx context.Context, ytClient yt.Client) (err error) {
	logger := log.FromContext(ctx)

	_, err = ytClient.CreateNode(
		ctx,
		ypath.Path("//sys/queue_agents/config"),
		yt.NodeDocument,
		&yt.CreateNodeOptions{
			Attributes: map[string]interface{}{
				"value": map[string]interface{}{
					"queue_agent": map[string]interface{}{
						"controller": map[string]interface{}{
							"enable_automatic_trimming": true,
						},
					},
					"cypress_synchronizer": map[string]interface{}{
						"policy":   "watching",
						"clusters": []string{qa.labeller.GetClusterName()},
					},
				},
			},
			Recursive:      true,
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating document '//sys/queue_agents/config' failed")
		return
	}

	err = ytClient.SetNode(
		ctx,
		ypath.Path("//sys/@cluster_connection/queue_agent"),
		map[string]interface{}{
			"stages": map[string]interface{}{
				"production": map[string]interface{}{
					"addresses": qa.cfgen.GetQueueAgentAddresses(),
				},
			},
			"queue_consumer_registration_manager": map[string]interface{}{
				"resolve_symlinks": true,
				"resolve_replicas": true,
			},
		},
		nil,
	)
	if err != nil {
		logger.Error(err, "Setting '//sys/@cluster_connection/queue_agent' failed")
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
		ypath.Path(fmt.Sprintf("//sys/clusters/%s", qa.labeller.GetClusterName())),
		clusterConnectionAttr,
		nil,
	)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Setting '//sys/clusters/%s' failed", qa.labeller.GetClusterName()))
		return
	}
	return
}

func (qa *queueAgent) prepareInitQueueAgentState() {
	path := "/usr/bin/init_queue_agent_state"

	script := []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf("if [[ -f \"%s\" ]]; then %s --create-registration-table --create-replicated-table-mapping-table --recursive --ignore-existing --proxy %s; fi",
			path, path, qa.cfgen.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole)),
	}

	qa.initQAState.SetInitScript(strings.Join(script, "\n"))
	job := qa.initQAState.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{qa.secret.GetEnvSource()}
}

func (qa *queueAgent) Status(ctx context.Context) ComponentStatus {
	status, err := qa.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (qa *queueAgent) Sync(ctx context.Context) error {
	_, err := qa.doSync(ctx, false)
	return err
}
