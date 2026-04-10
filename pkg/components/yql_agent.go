package components

import (
	"context"
	"fmt"
	"strings"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type YqlAgent struct {
	serverComponent

	cfgen             *ytconfig.Generator
	master            Component
	ytsaurusClient    internalYtsaurusClient
	initEnvironment   *InitJob
	updateEnvironment *InitJob
	secret            *resources.StringSecret
}

func NewYQLAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, yc internalYtsaurusClient, master Component) *YqlAgent {
	l := cfgen.GetComponentLabeller(consts.YqlAgentType, "")

	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.YQLAgents.InstanceSpec,
		"/usr/bin/ytserver-yql-agent",
		[]ConfigGenerator{{
			"ytserver-yql-agent.yson",
			ConfigFormatYson,
			func() ([]byte, error) { return cfgen.GetYQLAgentConfig(resource.Spec.YQLAgents) },
		}},
		consts.YQLAgentMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.YQLAgentRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &YqlAgent{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
		master:          master,
		ytsaurusClient:  yc,
		initEnvironment: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"yql-agent-environment",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&resource.Spec.YQLAgents.InstanceSpec,
		),
		updateEnvironment: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"yql-agent-update-environment",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&resource.Spec.YQLAgents.InstanceSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus),
	}
}

func (yqla *YqlAgent) GetFullName() string {
	return yqla.labeller.GetFullComponentName()
}

func (yqla *YqlAgent) GetShortName() string {
	return yqla.labeller.GetInstanceGroup()
}

func (yqla *YqlAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		yqla.server,
		yqla.initEnvironment,
		yqla.updateEnvironment,
		yqla.secret,
	)
}

func (yqla *YqlAgent) GetCypressPatch() ypatch.PatchSet {
	return ypatch.PatchSet{
		// NOTE: Patch for YtsaurusClient will copy "@cluster_connection" into "//sys/clusters".
		"//sys/@cluster_connection": {
			ypatch.Replace(
				"/yql_agent/stages/production/channel/addresses",
				yqla.cfgen.GetYQLAgentAddresses(),
			),
		},
	}
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
		fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection/yql_agent '{stages={production={channel={disable_balancing_on_single_address=%%false;addresses=%v}}}}'", yqlAgentAddrs),
		fmt.Sprintf("/usr/bin/yt get //sys/@cluster_connection | /usr/bin/yt set //sys/clusters/%s", yqla.labeller.GetClusterName()),
	}

	return strings.Join(script, "\n")
}

func (yqla *YqlAgent) createUpdateScript() ([]string, error) {
	return []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection/yql_agent/stages/production/channel/disable_balancing_on_single_address '%%false'"),
	}, nil
}

func (yqla *YqlAgent) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if yqla.ytsaurus.IsUpdating() {
		if !IsUpdatingComponent(yqla.ytsaurus, yqla) {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
		switch updateState := yqla.ytsaurus.GetUpdateState(); updateState {
		case ytv1.UpdateStateWaitingForPodsRemoval, ytv1.UpdateStateWaitingForPodsCreation:
			// Handle bulk update with pre-checks
			if status, err := handleBulkUpdatingClusterState(ctx, yqla.ytsaurus, yqla, &yqla.component, yqla.server, dry); status != nil {
				return *status, err
			}
		case ytv1.UpdateStateWaitingForYqlaUpdate:
			return yqla.updateEnvironment.RunUpdateScript(ctx, dry, yqla.ytsaurus, updateState, yqla.createUpdateScript, nil)
		default:
			return ComponentStatusReady(), nil
		}
	}

	if masterStatus := yqla.master.GetStatus(); !masterStatus.IsRunning() {
		return masterStatus.Blocker(), nil
	}

	if yqla.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := yqla.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = yqla.secret.Sync(ctx)
		}
		return ComponentStatusWaitingFor(yqla.secret.Name()), err
	}

	// TODO: Refactor this mess.
	if status, err := yqla.setup(ctx, dry); !status.IsReady() || err != nil {
		return status, err
	}

	if yqla.NeedSync() {
		if !dry {
			ss := yqla.server.buildStatefulSet()
			container := &ss.Spec.Template.Spec.Containers[0]
			container.Command = []string{"sh", "-c", fmt.Sprintf("echo -n $YT_TOKEN > %s; %s", consts.DefaultYqlTokenPath, strings.Join(container.Command, " "))}
			container.EnvFrom = []corev1.EnvFromSource{yqla.secret.GetEnvSource()}

			forceIPv4 := "0"
			forceIPv6 := "0"
			if yqla.ytsaurus.GetResource().Spec.UseIPv6 && !yqla.ytsaurus.GetResource().Spec.UseIPv4 {
				forceIPv6 = "1"
			} else if !yqla.ytsaurus.GetResource().Spec.UseIPv6 && yqla.ytsaurus.GetResource().Spec.UseIPv4 {
				forceIPv4 = "1"
			}
			container.Env = append(container.Env,
				corev1.EnvVar{Name: "YT_FORCE_IPV4", Value: forceIPv4},
				corev1.EnvVar{Name: "YT_FORCE_IPV6", Value: forceIPv6},
			)

			err = yqla.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if status, err := yqla.ArePodsReady(ctx); !status.IsReady() || err != nil {
		return status, err
	}

	// FIXME: Refactor this mess. During update flow sync must do only actions for current update phase.
	if yqla.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating && yqla.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsCreation {
		return ComponentStatusReady(), nil
	}

	return ComponentStatusReady(), nil
}

func (yqla *YqlAgent) setup(ctx context.Context, dry bool) (ComponentStatus, error) {
	if !dry {
		yqla.initEnvironment.SetInitScript(yqla.createInitScript())
	}

	status, err := yqla.initEnvironment.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	return yqla.updateEnvironment.RunScript(ctx, dry, "YQLAgentSetup", yqla.createUpdateScript, nil)
}

func (yqla *YqlAgent) UpdatePreCheck(ctx context.Context) ComponentStatus {
	// Get YT client from the ytsaurusClient component
	if yqla.ytsaurusClient == nil {
		return ComponentStatusBlocked("YtsaurusClient component is not available")
	}
	ytClient := yqla.ytsaurusClient.GetYtClient()

	// Check that the number of instances in YT matches the expected instanceCount
	if err := IsInstanceCountEqualYTSpec(ctx, ytClient, consts.YqlAgentType, yqla.server.getInstanceCount()); err != nil {
		return ComponentStatusBlocked("Error: %v", err)
	}

	return ComponentStatusReady()
}
