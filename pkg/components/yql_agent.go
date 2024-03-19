package components

import (
	"context"
	"fmt"
	"strings"

	"go.ytsaurus.tech/library/go/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
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
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelYqlAgent,
		ComponentName:  "YqlAgent",
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	if resource.Spec.YQLAgents.InstanceSpec.MonitoringPort == nil {
		resource.Spec.YQLAgents.InstanceSpec.MonitoringPort = ptr.Int32(consts.YQLAgentMonitoringPort)
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
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
	}
}

func (yqla *YqlAgent) IsUpdatable() bool {
	return true
}

func (yqla *YqlAgent) GetName() string {
	return yqla.labeller.ComponentName
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
		"/usr/bin/yt create document //sys/yql_agent/config --attributes '{}' --recursive --ignore-existing",
		fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection/yql_agent '{stages={production={channel={addresses=%v}}}}'", yqlAgentAddrs),
		fmt.Sprintf("/usr/bin/yt get //sys/@cluster_connection | /usr/bin/yt set //sys/clusters/%s", yqla.labeller.GetClusterName()),
	}

	return strings.Join(script, "\n")
}

func (yqla *YqlAgent) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(yqla.ytsaurus.GetClusterState()) && yqla.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if yqla.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, yqla.ytsaurus, yqla, &yqla.localComponent, yqla.server, true)
		if status != nil {
			if err != nil {
				panic(err)
			}
			return *status
		}
	}

	if !IsRunningStatus(yqla.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, yqla.master.GetName())
	}

	if yqla.secret.NeedSync(consts.TokenSecretKey, "") {
		return WaitingStatus(SyncStatusPending, yqla.secret.Name())
	}

	if yqla.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !yqla.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	st, err := yqla.initEnvironment.Sync(ctx, true)
	if err != nil {
		panic(err)
	}
	return st
}

func (yqla *YqlAgent) Sync(ctx context.Context) error {
	if ytv1.IsReadyToUpdateClusterState(yqla.ytsaurus.GetClusterState()) && yqla.server.needUpdate() {
		return nil
	}

	if yqla.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, yqla.ytsaurus, yqla, &yqla.localComponent, yqla.server, false)
		if status != nil {
			return err
		}
	}

	if !IsRunningStatus(yqla.master.Status(ctx).SyncStatus) {
		return nil
	}

	if yqla.secret.NeedSync(consts.TokenSecretKey, "") {
		s := yqla.secret.Build()
		s.StringData = map[string]string{
			consts.TokenSecretKey: ytconfig.RandString(30),
		}
		return yqla.secret.Sync(ctx)
	}

	if yqla.NeedSync() {
		ss := yqla.server.buildStatefulSet()
		container := &ss.Spec.Template.Spec.Containers[0]
		container.EnvFrom = []corev1.EnvFromSource{yqla.secret.GetEnvSource()}
		if yqla.ytsaurus.GetResource().Spec.UseIPv6 && !yqla.ytsaurus.GetResource().Spec.UseIPv4 {
			container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "0"}, {Name: "YT_FORCE_IPV6", Value: "1"}}
		} else if !yqla.ytsaurus.GetResource().Spec.UseIPv6 && yqla.ytsaurus.GetResource().Spec.UseIPv4 {
			container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "1"}, {Name: "YT_FORCE_IPV6", Value: "0"}}
		} else {
			container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "0"}, {Name: "YT_FORCE_IPV6", Value: "0"}}
		}
		return yqla.server.Sync(ctx)
	}

	if !yqla.server.arePodsReady(ctx) {
		return nil
	}

	yqla.initEnvironment.SetInitScript(yqla.createInitScript())

	_, err := yqla.initEnvironment.Sync(ctx, false)
	return err
}
