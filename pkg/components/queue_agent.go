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

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type QueueAgent struct {
	localServerComponent
	cfgen *ytconfig.Generator

	ytsaurusClient internalYtsaurusClient
	master         Component
	tabletNodes    []Component
	initCondition  string
	initQAState    *InitJob
	secret         *resources.StringSecret
}

func NewQueueAgent(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	yc internalYtsaurusClient,
	master Component,
	tabletNodes []Component,
) *QueueAgent {
	l := cfgen.GetComponentLabeller(consts.QueueAgentType, "")

	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.QueueAgents.InstanceSpec,
		"/usr/bin/ytserver-queue-agent",
		"ytserver-queue-agent.yson",
		func() ([]byte, error) { return cfgen.GetQueueAgentConfig(resource.Spec.QueueAgents) },
		consts.QueueAgentMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.QueueAgentRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &QueueAgent{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
		tabletNodes:          tabletNodes,
		initCondition:        "queueAgentInitCompleted",
		ytsaurusClient:       yc,
		initQAState: NewInitJob(
			l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"qa-state",
			consts.ClientConfigFileName,
			getImageWithDefault(resource.Spec.QueueAgents.InstanceSpec.Image, resource.Spec.CoreImage),
			cfgen.GetNativeClientConfig,
			getTolerationsWithDefault(resource.Spec.QueueAgents.Tolerations, resource.Spec.Tolerations),
			getNodeSelectorWithDefault(resource.Spec.QueueAgents.NodeSelector, resource.Spec.NodeSelector),
			getDNSConfigWithDefault(resource.Spec.QueueAgents.DNSConfig, resource.Spec.DNSConfig),
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus.APIProxy()),
	}
}

func (qa *QueueAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		qa.server,
		qa.initQAState,
		qa.secret,
	)
}

func (qa *QueueAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(qa.ytsaurus.GetClusterState()) && qa.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if qa.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, qa.ytsaurus, qa, &qa.localComponent, qa.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := qa.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, qa.master.GetFullName()), err
	}

	// It makes no sense to start queue agents without tablet nodes.
	if len(qa.tabletNodes) == 0 {
		return WaitingStatus(SyncStatusBlocked, "tablet nodes"), fmt.Errorf("cannot initialize queue agent without tablet nodes")
	}
	for _, tnd := range qa.tabletNodes {
		tndStatus, err := tnd.Status(ctx)
		if err != nil {
			return tndStatus, err
		}
		if !IsRunningStatus(tndStatus.SyncStatus) {
			return WaitingStatus(SyncStatusBlocked, tnd.GetFullName()), err
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
		ytClientStatus, err := qa.ytsaurusClient.Status(ctx)
		if err != nil {
			return ytClientStatus, err
		}
		if ytClientStatus.SyncStatus != SyncStatusReady {
			return WaitingStatus(SyncStatusBlocked, qa.ytsaurusClient.GetFullName()), err
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
			return WaitingStatus(SyncStatusPending, fmt.Sprintf("%s initialization", qa.GetFullName())), err
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

func (qa *QueueAgent) createUser(ctx context.Context, ytClient yt.Client) (err error) {
	logger := log.FromContext(ctx)

	token, _ := qa.secret.GetValue(consts.TokenSecretKey)
	err = CreateUser(ctx, ytClient, "queue_agent", token, true)
	if err != nil {
		logger.Error(err, "Creating user 'queue_agent' failed")
		return
	}
	return
}

func (qa *QueueAgent) init(ctx context.Context, ytClient yt.Client) (err error) {
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

func (qa *QueueAgent) prepareInitQueueAgentState() {
	path := "/usr/bin/init_queue_agent_state"
	proxy := qa.cfgen.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole)

	// Somewhere in 24.1 this script has changed signature and since it is not tied to some version we can check
	// we will try to call it new way and fallback to old way on error.
	// COMPAT(l0kix2): Remove after 23.1 not supported in the yt operator.
	oldVersionInvokation := fmt.Sprintf("%s --create-registration-table --create-replicated-table-mapping-table --recursive --ignore-existing --proxy %s",
		path,
		proxy,
	)
	newVersionInvokation := fmt.Sprintf("%s --latest --proxy %s",
		path,
		proxy,
	)

	script := []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf(`if [ ! -f %s ]; then`, path),
		fmt.Sprintf(`echo "%s doesn't exist, nothing to do"`, path),
		`exit 0`,
		`fi`,
		// Temporary turning off exiting on non-zero status, since we expect this command may fail on
		// unexpected arguments in the older server versions.
		// In case arguments are valid and other error occurs it is not a problem, since new binary will fail with
		// the old arguments later anyway.
		`set +e`,
		newVersionInvokation,
		`if [ $? -ne 0 ]; then`,
		`set -e`,
		`echo "Binary execution failed. Running with an old set of arguments"`,
		oldVersionInvokation,
		`fi`,
	}

	qa.initQAState.SetInitScript(strings.Join(script, "\n"))
	job := qa.initQAState.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{qa.secret.GetEnvSource()}
}

func (qa *QueueAgent) Status(ctx context.Context) (ComponentStatus, error) {
	return qa.doSync(ctx, true)
}

func (qa *QueueAgent) Sync(ctx context.Context) error {
	_, err := qa.doSync(ctx, false)
	return err
}
