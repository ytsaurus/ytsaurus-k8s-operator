package components

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type YqlAgent struct {
	localServerComponent
	cfgen           *ytconfig.Generator
	master          Component
	initEnvironment *InitJob
	secret          *resources.StringSecret
}

func NewYQLAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component) *YqlAgent {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:    &resource.ObjectMeta,
		APIProxy:      ytsaurus.APIProxy(),
		ComponentType: consts.YqlAgentType,
		Annotations:   resource.Spec.ExtraPodAnnotations,
	}

	if resource.Spec.YQLAgents.InstanceSpec.MonitoringPort == nil {
		resource.Spec.YQLAgents.InstanceSpec.MonitoringPort = ptr.To(int32(consts.YQLAgentMonitoringPort))
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.YQLAgents.InstanceSpec,
		"/usr/bin/ytserver-yql-agent",
		"ytserver-yql-agent.yson",
		cfgen.GetYQLAgentStatefulSetName(),
		cfgen.GetYQLAgentServiceName(),
		func() ([]byte, error) {
			return cfgen.GetYQLAgentConfig(resource.Spec.YQLAgents)
		},
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.YQLAgentRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &YqlAgent{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
		initEnvironment: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"yql-agent-environment",
			consts.ClientConfigFileName,
			getImageWithDefault(resource.Spec.YQLAgents.Image, resource.Spec.CoreImage),
			cfgen.GetNativeClientConfig,
			getTolerationsWithDefault(resource.Spec.YQLAgents.Tolerations, resource.Spec.Tolerations),
			getNodeSelectorWithDefault(resource.Spec.YQLAgents.NodeSelector, resource.Spec.NodeSelector),
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
	}
}

func (yqla *YqlAgent) IsUpdatable() bool {
	return true
}

func (yqla *YqlAgent) GetType() consts.ComponentType { return consts.YqlAgentType }

func (yqla *YqlAgent) GetName() string {
	return yqla.labeller.GetFullComponentName()
}

func (yqla *YqlAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		yqla.server,
		yqla.initEnvironment,
		yqla.secret,
	)
}

func (yqla *YqlAgent) initUsers() string {
	token, _ := yqla.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand(consts.YqlUserName, "", token, true)
	commands = append(commands, createUserCommand("yql_agent", "", "", true)...)
	return strings.Join(commands, "\n")
}

func (yqla *YqlAgent) createInitScript() string {
	var sb strings.Builder
	sb.WriteString("[")
	for _, addr := range yqla.cfgen.GetYQLAgentAddresses() {
		sb.WriteString("\"")
		sb.WriteString(addr)
		sb.WriteString("\";")
	}
	sb.WriteString("]")
	yqlAgentAddrs := sb.String()
	script := []string{
		initJobWithNativeDriverPrologue(),
		yqla.initUsers(),
		"/usr/bin/yt add-member --member yql_agent --group superusers || true",
		"/usr/bin/yt create document //sys/yql_agent/config --attributes '{value={}}' --recursive --ignore-existing",
		fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection/yql_agent '{stages={production={channel={addresses=%v}}}}'", yqlAgentAddrs),
		fmt.Sprintf("/usr/bin/yt get //sys/@cluster_connection | /usr/bin/yt set //sys/clusters/%s", yqla.labeller.GetClusterName()),
	}

	return strings.Join(script, "\n")
}

func (yqla *YqlAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(yqla.ytsaurus.GetClusterState()) && yqla.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if yqla.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, yqla.ytsaurus, yqla, &yqla.localComponent, yqla.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := yqla.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, yqla.master.GetName()), err
	}

	if yqla.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := yqla.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = yqla.secret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, yqla.secret.Name()), err
	}

	if yqla.NeedSync() {
		if !dry {
			ss := yqla.server.buildStatefulSet()
			container := &ss.Spec.Template.Spec.Containers[0]
			container.Command = []string{"sh", "-c", fmt.Sprintf("echo -n $YT_TOKEN > %s; %s", consts.DefaultYqlTokenPath, strings.Join(container.Command, " "))}
			container.EnvFrom = []corev1.EnvFromSource{yqla.secret.GetEnvSource()}
			if yqla.ytsaurus.GetResource().Spec.UseIPv6 && !yqla.ytsaurus.GetResource().Spec.UseIPv4 {
				container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "0"}, {Name: "YT_FORCE_IPV6", Value: "1"}}
			} else if !yqla.ytsaurus.GetResource().Spec.UseIPv6 && yqla.ytsaurus.GetResource().Spec.UseIPv4 {
				container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "1"}, {Name: "YT_FORCE_IPV6", Value: "0"}}
			} else {
				container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "0"}, {Name: "YT_FORCE_IPV6", Value: "0"}}
			}
			err = yqla.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !yqla.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	if !dry {
		yqla.initEnvironment.SetInitScript(yqla.createInitScript())
	}

	return yqla.initEnvironment.Sync(ctx, dry)
}

func (yqla *YqlAgent) Status(ctx context.Context) (ComponentStatus, error) {
	return yqla.doSync(ctx, true)
}

func (yqla *YqlAgent) Sync(ctx context.Context) error {
	_, err := yqla.doSync(ctx, false)
	return err
}
