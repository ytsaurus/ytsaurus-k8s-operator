package components

import (
	"context"
	"fmt"
	"strings"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	corev1 "k8s.io/api/core/v1"
)

type yqlAgent struct {
	componentBase
	server          server
	master          Component
	initEnvironment *InitJob
	secret          *resources.StringSecret
}

func NewYQLAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelYqlAgent,
		ComponentName:  "YqlAgent",
		MonitoringPort: consts.YQLAgentMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	server := newServer(
		&l,
		ytsaurus,
		&resource.Spec.YQLAgents.InstanceSpec,
		"/usr/bin/ytserver-yql-agent",
		"ytserver-yql-agent.yson",
		cfgen.GetYQLAgentStatefulSetName(),
		cfgen.GetYQLAgentServiceName(),
		cfgen.GetYQLAgentConfig,
	)

	return &yqlAgent{
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		server: server,
		master: master,
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

func (yqla *yqlAgent) IsUpdatable() bool {
	return true
}

func (yqla *yqlAgent) GetName() string {
	return yqla.labeller.ComponentName
}

func (yqla *yqlAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		yqla.server,
		yqla.initEnvironment,
		yqla.secret,
	})
}

func (yqla *yqlAgent) initUsers() string {
	token, _ := yqla.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand(consts.YqlUserName, "", token, true)
	commands = append(commands, createUserCommand("yql_agent", "", "", true)...)
	return strings.Join(commands, "\n")
}

func (yqla *yqlAgent) createInitScript() string {
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

func (yqla *yqlAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(yqla.ytsaurus.GetClusterState()) && yqla.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if yqla.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, yqla.ytsaurus, yqla, &yqla.componentBase, yqla.server, dry); status != nil {
			return *status, err
		}
	}

	if !IsRunningStatus(yqla.master.Status(ctx).SyncStatus) {
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

	if yqla.server.needSync() {
		if !dry {
			ss := yqla.server.buildStatefulSet()
			container := &ss.Spec.Template.Spec.Containers[0]
			container.EnvFrom = []corev1.EnvFromSource{yqla.secret.GetEnvSource()}
			if yqla.ytsaurus.GetResource().Spec.UseIPv6 {
				container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "0"}, {Name: "YT_FORCE_IPV6", Value: "1"}}
			} else {
				container.Env = []corev1.EnvVar{{Name: "YT_FORCE_IPV4", Value: "1"}, {Name: "YT_FORCE_IPV6", Value: "0"}}
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

func (yqla *yqlAgent) Status(ctx context.Context) ComponentStatus {
	status, err := yqla.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (yqla *yqlAgent) Sync(ctx context.Context) error {
	_, err := yqla.doSync(ctx, false)
	return err
}
