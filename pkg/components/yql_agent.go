package components

import (
	"context"
	"fmt"
	"strings"

	"go.ytsaurus.tech/yt/go/yt"

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

	cfgen           *ytconfig.Generator
	ytsaurusClient  internalYtsaurusClient
	initEnvironment *InitJob
	secret          *resources.StringSecret
	execSecret      *resources.StringSecret
}

func NewYQLAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, yc internalYtsaurusClient) *YqlAgent {
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

	var execSecret *resources.StringSecret
	if resource.Spec.YQLAgents.DQEngine != nil {
		execSecret = resources.NewStringSecret(l.GetSidecarSecretName("exec"), l, ytsaurus)
	}

	return &YqlAgent{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
		ytsaurusClient:  yc,
		initEnvironment: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"yql-agent-environment",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&resource.Spec.YQLAgents.InstanceSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus),
		execSecret: execSecret,
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
		yqla.secret,
		yqla.execSecret,
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

func (yqla *YqlAgent) scriptDQInit() TextGeneratorFunc {
	if yqla.execSecret == nil {
		return PlainTextGenerator()
	}
	dqDirectoryACE := yt.ACE{
		Action:          yt.ActionAllow,
		Subjects:        []string{consts.YQLAgentExecUserName},
		Permissions:     []yt.Permission{yt.PermissionCreate, yt.PermissionRead, yt.PermissionRemove, yt.PermissionWrite},
		InheritanceMode: yt.InheritanceModeObjectAndDescendants,
	}
	sysAccountACE := yt.ACE{
		Action:          yt.ActionAllow,
		Subjects:        []string{consts.YQLAgentExecUserName},
		Permissions:     []yt.Permission{yt.PermissionUse},
		InheritanceMode: yt.InheritanceModeObjectAndDescendants,
	}
	return JoinTextGenerators(
		PlainTextGenerator("/usr/bin/yt create map_node //sys/yql_agent/dq --ignore-existing"),
		CheckAndAppendPathACLGenerator("//sys/yql_agent/dq", dqDirectoryACE),
		CheckAndAppendPathACLGenerator("//sys/accounts/sys", sysAccountACE),
	)
}

func (yqla *YqlAgent) createInitScript() TextGeneratorFunc {
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
		"/usr/bin/yt create document //sys/yql_agent/config --attributes '{value={}}' --recursive --ignore-existing",
		fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection/yql_agent '{stages={production={channel={disable_balancing_on_single_address=%%false;addresses=%v}}}}'", yqlAgentAddrs),
		fmt.Sprintf("/usr/bin/yt get //sys/@cluster_connection | /usr/bin/yt set //sys/clusters/%s", yqla.labeller.GetClusterName()),
	}
	return JoinTextGenerators(
		PlainTextGenerator(script...),
		yqla.scriptDQInit(),
	)
}

func (yqla *YqlAgent) createUpdateScript() TextGeneratorFunc {
	script := []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection/yql_agent/stages/production/channel/disable_balancing_on_single_address '%%false'"),
	}
	return JoinTextGenerators(
		PlainTextGenerator(script...),
		yqla.scriptDQInit(),
	)
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
			return yqla.initEnvironment.RunUpdateScript(ctx, dry, yqla.ytsaurus, updateState, yqla.createUpdateScript(), nil)
		default:
			return ComponentStatusReady(), nil
		}
	}

	if status, err := SyncUserToken(ctx, yqla.ytsaurusClient, yqla.secret, consts.YQLAgentUserName, consts.SuperusersGroupName, dry); err != nil || !status.IsRunning() {
		return status, err
	}

	if yqla.execSecret != nil {
		if status, err := SyncUserToken(ctx, yqla.ytsaurusClient, yqla.execSecret, consts.YQLAgentExecUserName, "", dry); err != nil || !status.IsRunning() {
			return status, err
		}
	}

	if yqla.ytsaurus.IsReadyForInitJobs() && !yqla.initEnvironment.IsCompleted() {
		if status, err := yqla.initEnvironment.RunScript(ctx, dry, "YQLAgentInit", yqla.createInitScript(), nil); err != nil || !status.IsReady() {
			return status, err
		}
	}

	if yqla.NeedSync() {
		if !dry {
			ss := yqla.server.buildStatefulSet()

			podSpec := &ss.Spec.Template.Spec
			container := &podSpec.Containers[0]

			podSpec.Volumes = append(podSpec.Volumes, yqla.secret.GetTokenVolume(consts.YQLAgentTokenVolumeName))
			container.VolumeMounts = append(container.VolumeMounts, yqla.secret.GetTokenVolumeMount(consts.YQLAgentTokenVolumeName))

			if yqla.execSecret != nil {
				podSpec.Volumes = append(podSpec.Volumes, yqla.execSecret.GetTokenVolume(consts.YQLExecTokenVolumeName))
				container.VolumeMounts = append(container.VolumeMounts, yqla.execSecret.GetTokenVolumeMount(consts.YQLExecTokenVolumeName))
			}

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
