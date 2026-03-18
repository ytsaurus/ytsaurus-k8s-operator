package components

import (
	"context"
	"fmt"
	"strings"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	defaultHostAddressLabel = "kubernetes.io/hostname"
	mediumInitQuota         = 1 << 30 // enough to start the cluster
)

type Master struct {
	serverComponent

	mastersSpec *ytv1.MastersSpec

	cfgen *ytconfig.Generator

	initJob *InitJob

	adminCredentials corev1.Secret

	uploaderSecret *resources.StringSecret
}

func buildMasterOptions(mastersSpec *ytv1.MastersSpec) []Option {
	options := []Option{
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.MasterRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
		WithReadinessByContainer(consts.YTServerContainerName),
	}

	if mastersSpec.HydraPersistenceUploader != nil && mastersSpec.HydraPersistenceUploader.Image != nil {
		options = append(options, WithSidecarImage(
			consts.HydraPersistenceUploaderContainerName,
			*mastersSpec.HydraPersistenceUploader.Image,
		))
	}

	checkAndAddTimbertruckToServerOptions(
		&options,
		mastersSpec.Timbertruck,
		mastersSpec.InstanceSpec.StructuredLoggers,
	)

	return options
}

func NewMaster(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	mastersSpec *ytv1.MastersSpec,
) *Master {
	l := cfgen.GetComponentLabeller(consts.MasterType, "")

	srv := newServer(
		l,
		ytsaurus,
		&mastersSpec.InstanceSpec,
		"/usr/bin/ytserver-master",
		[]ConfigGenerator{
			{
				"ytserver-master.yson",
				ConfigFormatYson,
				func() ([]byte, error) { return cfgen.GetMasterConfig(mastersSpec) },
			},
			{
				consts.ClientConfigFileName,
				ConfigFormatYson,
				cfgen.GetNativeClientConfig,
			},
		},
		consts.MasterMonitoringPort,
		buildMasterOptions(mastersSpec)...,
	)

	initJob := NewInitJobForYtsaurus(
		l,
		ytsaurus,
		"default",
		consts.ClientConfigFileName,
		cfgen.GetNativeClientConfig,
		&mastersSpec.InstanceSpec,
	)

	var uploaderSecret *resources.StringSecret
	if mastersSpec.HydraPersistenceUploader != nil {
		uploaderSecret = resources.NewStringSecret(buildUserCredentialsSecretname(consts.HydraPersistenceUploaderUserName), l, ytsaurus)
	}

	return &Master{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		mastersSpec:     mastersSpec,
		cfgen:           cfgen,
		initJob:         initJob,
		uploaderSecret:  uploaderSecret,
	}
}

func (m *Master) Fetch(ctx context.Context) error {
	if m.ytsaurus.GetResource().Spec.AdminCredentials != nil {
		err := m.ytsaurus.FetchObject(
			ctx,
			m.ytsaurus.GetResource().Spec.AdminCredentials.Name,
			&m.adminCredentials)
		if err != nil {
			return err
		}
	}

	return resources.Fetch(ctx,
		m.server,
		m.initJob,
		m.uploaderSecret,
	)
}

func (m *Master) getAdminCredentials() (adminLogin string, adminPassword string, adminToken string) {
	adminLogin, adminPassword = consts.DefaultAdminLogin, consts.DefaultAdminPassword
	adminToken = consts.DefaultAdminPassword

	if m.adminCredentials.Name != "" {
		value, ok := m.adminCredentials.Data[consts.AdminLoginSecret]
		if ok {
			adminLogin = string(value)
		}
		value, ok = m.adminCredentials.Data[consts.AdminPasswordSecret]
		if ok {
			adminPassword = string(value)
		}

		value, ok = m.adminCredentials.Data[consts.AdminTokenSecret]
		if ok {
			adminToken = string(value)
		}
	}
	return adminLogin, adminPassword, adminToken
}

func (m *Master) initAdminUser() string {
	adminLogin, adminPassword, adminToken := m.getAdminCredentials()

	commands := createUserCommand(adminLogin, adminPassword, adminToken, true)
	return RunIfNonexistent(fmt.Sprintf("//sys/users/%s", adminLogin), commands...)
}

func (m *Master) initUploaderUser() (string, error) {
	if m.uploaderSecret == nil {
		return "", nil
	}

	login := consts.HydraPersistenceUploaderUserName
	token, _ := m.uploaderSecret.GetValue(consts.TokenSecretKey)
	commands := []string{
		strings.Join(createUserCommand(login, token, token, false), "\n"),
	}

	setPathAclCommand, err := SetPathAcl("//sys/admin/snapshots", []yt.ACE{
		{
			Action:          "allow",
			Subjects:        []string{login},
			Permissions:     []yt.Permission{"read", "write", "remove", "create"},
			InheritanceMode: "object_and_descendants",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create set acl command: %w", err)
	}
	appendPathAclCommand, err := AppendPathAcl("//sys/accounts/sys", yt.ACE{
		Action:      "allow",
		Subjects:    []string{login},
		Permissions: []yt.Permission{"use"},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create append acl command: %w", err)
	}

	commands = append(commands,
		"/usr/bin/yt create map_node //sys/admin/snapshots -r -i",
		setPathAclCommand,
		appendPathAclCommand,
	)
	return RunIfCondition(
		fmt.Sprintf("'%v' = 'true'", m.mastersSpec.HydraPersistenceUploader != nil),
		RunIfNonexistent(fmt.Sprintf("//sys/users/%s", login), commands...),
	), nil
}

type Medium struct {
	Name string `yson:"name"`
}

func (m *Master) getExtraMedia() []Medium {
	mediaMap := make(map[string]Medium)

	for _, d := range m.ytsaurus.GetResource().Spec.DataNodes {
		for _, l := range d.Locations {
			if l.Medium == consts.DefaultMedium {
				continue
			}
			mediaMap[l.Medium] = Medium{
				Name: l.Medium,
			}
		}
	}

	mediaSlice := make([]Medium, 0, len(mediaMap))
	for _, v := range mediaMap {
		mediaSlice = append(mediaSlice, v)
	}

	return mediaSlice
}

func (m *Master) initMedia() string {
	var commands []string
	for _, medium := range m.getExtraMedia() {
		attr, err := yson.MarshalFormat(medium, yson.FormatText)
		if err != nil {
			panic(err)
		}
		// COMPAT(gritukan): Remove "medium" after some time.
		commands = append(commands, fmt.Sprintf("/usr/bin/yt get //sys/media/%s/@name || /usr/bin/yt create domestic_medium --attr '%s' || /usr/bin/yt create medium --attr '%s'", medium.Name, string(attr), string(attr)))

		quotaPath := fmt.Sprintf("//sys/accounts/sys/@resource_limits/disk_space_per_medium/%s", medium.Name)
		commands = append(commands, fmt.Sprintf("/usr/bin/yt get %s || /usr/bin/yt set %s %d", quotaPath, quotaPath, mediumInitQuota))
	}
	return strings.Join(commands, "\n")
}

func (m *Master) initGroups() string {
	commands := []string{
		"/usr/bin/yt create group --attr '{name=admins}' --ignore-existing",
	}
	return strings.Join(commands, "\n")
}

func (m *Master) initSchemaACLs() (string, error) {
	userReadACE := yt.ACE{
		Action:      "allow",
		Subjects:    []string{"users"},
		Permissions: []yt.Permission{"read"},
	}
	userReadCreateACE := yt.ACE{
		Action:      "allow",
		Subjects:    []string{"users"},
		Permissions: []yt.Permission{"read", "create"},
	}
	userReadWriteCreateACE := yt.ACE{
		Action:      "allow",
		Subjects:    []string{"users"},
		Permissions: []yt.Permission{"read", "write", "create"},
	}

	adminACE := yt.ACE{
		Action:      "allow",
		Subjects:    []string{"admins"},
		Permissions: []yt.Permission{"read", "write", "administer", "create", "remove"},
	}

	var commands []string

	// Users should not be able to create or write objects of these types on their own.
	for _, objectType := range []string{
		"tablet_cell", "tablet_action", "tablet_cell_bundle",
		"user", "group",
		"rack", "data_center", "cluster_node",
		"access_control_object_namespace", "access_control_object_namespace_map"} {
		setPathAclCommand, err := SetPathAcl(fmt.Sprintf("//sys/schemas/%s", objectType), []yt.ACE{
			userReadACE,
			adminACE,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create set acl command for %s: %w", objectType, err)
		}
		commands = append(commands, setPathAclCommand)
	}
	// COMPAT(achulkov2): Drop the first command after `medium` is obsolete in all major versions.

	setPathAclMediumCommand, err := SetPathAcl("//sys/schemas/medium", []yt.ACE{
		userReadACE,
		adminACE,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create set acl command for medium: %w", err)
	}

	setPathAclDomesticMediumCommand, err := SetPathAcl("//sys/schemas/domestic_medium", []yt.ACE{
		userReadCreateACE,
		adminACE,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create set acl command for domestic_medium: %w", err)
	}

	commands = append(commands, fmt.Sprintf("%s || %s",
		setPathAclMediumCommand,
		setPathAclDomesticMediumCommand))

	// Users can create pools, pool trees, accounts and access control objects given the right circumstances and permissions.
	for _, objectType := range []string{"account", "scheduler_pool", "scheduler_pool_tree", "access_control_object"} {
		setPathAclCommand, err := SetPathAcl(fmt.Sprintf("//sys/schemas/%s", objectType), []yt.ACE{
			userReadCreateACE,
			adminACE,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create set acl command for %s: %w", objectType, err)
		}
		commands = append(commands, setPathAclCommand)
	}

	// Users can write account_resource_usage_lease objects.
	setPathAclCommand, err := SetPathAcl("//sys/schemas/account_resource_usage_lease", []yt.ACE{
		userReadWriteCreateACE,
		adminACE,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create set acl command for account_resource_usage_lease: %w", err)
	}
	commands = append(commands, setPathAclCommand)

	return strings.Join(commands, "\n"), nil
}

func (m *Master) scriptInitialization() ([]string, error) {
	clusterConn := m.cfgen.GetClusterConnection()
	connConfig, err := yson.MarshalFormat(clusterConn, yson.FormatPretty)
	if err != nil {
		return nil, err
	}

	initSchemaACLsCommands, err := m.initSchemaACLs()
	if err != nil {
		return nil, fmt.Errorf("failed to create init schema ACLs commands: %w", err)
	}

	initHydraPersistenceUploaderUserCommands, err := m.initUploaderUser()
	if err != nil {
		return nil, fmt.Errorf("failed to create init hydra persistence uploader user commands: %w", err)
	}

	initCommands := []string{
		m.initGroups(),
		RunIfExists("//sys/@provision_lock", initSchemaACLsCommands),
		"/usr/bin/yt create scheduler_pool_tree --attributes '{name=default; config={nodes_filter=\"\"}}' --ignore-existing",
		SetWithIgnoreExisting("//sys/pool_trees/@default_tree", "default"),
		RunIfNonexistent("//sys/pools", "/usr/bin/yt link //sys/pool_trees/default //sys/pools"),
		RunIfNonexistent("//sys/pool_trees/default/research", "/usr/bin/yt create scheduler_pool --attributes '{name=research; pool_tree=default}'"),
		"/usr/bin/yt create map_node //home --ignore-existing",
		RunIfExists("//sys/@provision_lock", fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection '%s'", string(connConfig))),
		RunIfExists("//sys/@provision_lock", fmt.Sprintf("/usr/bin/yt set //sys/@cluster_name '%s'", clusterConn.ClusterName)),
		m.initAdminUser(),
		m.initMedia(),
	}

	initScript := RunIfCondition(
		fmt.Sprintf("'%v' = 'true'", ytv1.ClusterStateInitializing == m.ytsaurus.GetClusterState()),
		initCommands...,
	)

	script := []string{
		initJobWithNativeDriverPrologue(),
		initScript,
		initHydraPersistenceUploaderUserCommands,
		"/usr/bin/yt remove //sys/@provision_lock -f",
	}

	return script, nil
}

func (m *Master) scriptExitReadOnly() ([]string, error) {
	return []string{
		initJobWithNativeDriverPrologue(),
		"export YT_LOG_LEVEL=DEBUG",
		// COMPAT(l0kix2): remove || part when the compatibility with 23.1 and older is dropped.
		`[[ "$YTSAURUS_VERSION" < "23.2" ]] && echo "master_exit_read_only is supported since 23.2, nothing to do" && exit 0`,
		"/usr/bin/yt execute master_exit_read_only '{}'",
	}, nil
}

func (m *Master) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if m.ytsaurus.IsUpdating() {
		if !IsUpdatingComponent(m.ytsaurus, m) {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}

		updateState := m.ytsaurus.GetUpdateState()

		switch updateState {
		case ytv1.UpdateStateWaitingForMasterExitReadOnly:
			return m.runUpdateScript(ctx, dry, updateState, m.scriptExitReadOnly, nil)
		case ytv1.UpdateStateWaitingForSidecarsInitialize:
			return m.runUpdateScript(ctx, dry, updateState, m.scriptInitialization, nil)
		case ytv1.UpdateStateWaitingForPodsRemoval, ytv1.UpdateStateWaitingForPodsCreation:
			// TODO: Cleanup, add separate update states for strategies.
			switch getComponentUpdateStrategy(m.ytsaurus, consts.MasterType, m.GetShortName()) {
			case ytv1.ComponentUpdateModeTypeOnDelete:
				if status, err := handleOnDeleteUpdatingClusterState(ctx, m.ytsaurus, m, &m.component, m.server, dry); status != nil {
					return *status, err
				}
				return ComponentStatusReady(), err
			default:
				if status, err := handleBulkUpdatingClusterState(ctx, m.ytsaurus, m, &m.component, m.server, dry); status != nil {
					return *status, err
				}
			}
		default:
			return ComponentStatusReadyAfter("No actions required for this update state"), nil
		}
	}

	if m.uploaderSecret != nil && m.uploaderSecret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			token := ytconfig.RandString(30)
			s := m.uploaderSecret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: token,
			}
			err = m.uploaderSecret.Sync(ctx)
		}
		return ComponentStatusWaitingFor(m.uploaderSecret.Name()), err
	}

	if m.NeedSync() {
		if !dry {
			err = m.doServerSync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !m.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	if m.ytsaurus.IsInitializing() {
		return m.runInitPhaseJobs(ctx, dry)
	}

	return ComponentStatusReady(), nil
}

func (m *Master) doServerSync(ctx context.Context) error {
	statefulSet := m.server.buildStatefulSet()
	podSpec := &statefulSet.Spec.Template.Spec

	podSpec.Containers[0].Env = append(podSpec.Containers[0].Env, getNativeClientConfigEnv()...)

	if m.mastersSpec.HydraPersistenceUploader != nil && m.mastersSpec.HydraPersistenceUploader.Image != nil {
		addHydraPersistenceUploaderToPodSpec(
			*m.mastersSpec.HydraPersistenceUploader.Image,
			podSpec,
			m.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
			m.uploaderSecret.Name(),
		)
	}
	if err := checkAndAddTimbertruckToPodSpec(m.mastersSpec.Timbertruck, podSpec, &m.mastersSpec.InstanceSpec, m.labeller, m.cfgen); err != nil {
		return err
	}
	if err := AddSidecarsToPodSpec(m.mastersSpec.Sidecars, podSpec); err != nil {
		return err
	}

	if len(m.mastersSpec.HostAddresses) != 0 {
		AddAffinity(statefulSet, m.getHostAddressLabel(), m.mastersSpec.HostAddresses)
	}
	return m.server.Sync(ctx)
}

func (m *Master) GetCypressPatch() ypatch.PatchSet {
	clusterConnection := m.cfgen.GetClusterConnection()
	return ypatch.PatchSet{
		"//sys/@cluster_connection": {
			ypatch.Replace("/primary_master/addresses", &clusterConnection.PrimaryMaster.Addresses),
			ypatch.Replace("/primary_master/peers", &clusterConnection.PrimaryMaster.Peers),
			ypatch.ReplaceOrRemove("/bus_client", clusterConnection.BusClient),
		},
	}
}

func (m *Master) getHostAddressLabel() string {
	if m.mastersSpec.HostAddressLabel != "" {
		return m.mastersSpec.HostAddressLabel
	}
	return defaultHostAddressLabel
}

func (m *Master) runInitPhaseJobs(ctx context.Context, dry bool) (ComponentStatus, error) {
	return m.initJob.RunScript(ctx, dry, "ClusterInitialization", m.scriptInitialization, func() {})
}

func (m *Master) runUpdateScript(
	ctx context.Context,
	dry bool,
	updateState ytv1.UpdateState,
	script func() ([]string, error),
	complete func(),
) (ComponentStatus, error) {
	return m.initJob.RunScript(ctx, dry, string(updateState), script, func() {
		message := fmt.Sprintf("Job %s for %s is complete", m.initJob.Name(), updateState)
		m.ytsaurus.LogUpdate(ctx, message)
		if complete != nil {
			complete()
		}
		m.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    m.ytsaurus.GetUpdateStateCompleteCondition(updateState),
			Status:  metav1.ConditionTrue,
			Reason:  "JobComplete",
			Message: message,
		})
	})
}

func addHydraPersistenceUploaderToPodSpec(hydraImage string, podSpec *corev1.PodSpec, proxy string, secretKey string) {
	podSpec.Containers = append(podSpec.Containers,
		corev1.Container{
			Name:    consts.HydraPersistenceUploaderContainerName,
			Image:   hydraImage,
			Command: []string{"/usr/bin/hydra_persistence_uploader"},
			Env: append([]corev1.EnvVar{
				{
					Name: consts.TokenSecretKey,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretKey,
							},
							Key: consts.TokenSecretKey,
						},
					},
				},
				{Name: "YT_PROXY", Value: proxy},
			}, getDefaultEnv()...),
			VolumeMounts: []corev1.VolumeMount{
				{Name: consts.ConfigTemplateVolumeName, MountPath: consts.ConfigMountPoint, ReadOnly: true},
				{Name: "master-data", MountPath: "/yt/master-data", ReadOnly: true},
				{Name: "master-logs", MountPath: "/yt/master-logs", ReadOnly: true},
				{Name: "shared-binaries", MountPath: "/shared-binaries", ReadOnly: false},
			},
			ImagePullPolicy: corev1.PullIfNotPresent,
		},
	)

	command := strings.Join([]string{
		"rm /shared-binaries/*",
		"cp /usr/bin/ytserver-all /shared-binaries/ytserver-all",
		"ln /shared-binaries/ytserver-all /shared-binaries/ytserver-master",
	}, "; ")
	backgroundCommand := fmt.Sprintf("nohup bash -c '%s' > /dev/null 2>&1 &", command)
	for i := range podSpec.Containers {
		if podSpec.Containers[i].Name == "ytserver" {
			podSpec.Containers[i].Lifecycle = &corev1.Lifecycle{
				PostStart: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"/bin/bash", "-c", backgroundCommand},
					},
				},
			}
			podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts,
				corev1.VolumeMount{
					Name:      "shared-binaries",
					MountPath: "/shared-binaries",
				},
			)
			break
		}
	}

	podSpec.Volumes = append(podSpec.Volumes,
		corev1.Volume{
			Name: "shared-binaries",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	)
}

// NYT::NHydra::EPeerState
type MasterState string

const (
	MasterStateLeading   MasterState = "leading"
	MasterStateFollowing MasterState = "following"
)

// See: https://github.com/ytsaurus/ytsaurus/blob/main/yt/yt/server/lib/hydra/distributed_hydra_manager.cpp
type MasterHydra struct {
	State                MasterState `yson:"state"`
	SelfID               int32       `yson:"self_id"`
	LeaderID             int32       `yson:"leader_id"`
	Voting               bool        `yson:"voting"`
	Active               bool        `yson:"active"`
	ActiveLeader         bool        `yson:"active_leader"`
	ActiveFollower       bool        `yson:"active_follower"`
	BuildingSnapshot     bool        `yson:"building_snapshot"`
	EnteringReadOnlyMode bool        `yson:"entering_read_only_mode"`
	LastSnapshotReadOnly bool        `yson:"last_snapshot_read_only"`
	ReadOnly             bool        `yson:"read_only"`
}

type MasterAddressWithAttributes struct {
	Address     string `yson:",value"`
	Maintenance bool   `yson:"maintenance,attr"`
}

func (m *Master) CheckQuorumHealth(ctx context.Context, ytClient yt.Client) (ok bool, msg string, err error) {
	mastersPath := ypath.Path(consts.PrimaryMastersPath)

	totalCount := m.server.getInstanceCount()
	requiredCount := m.server.getMinReadyInstanceCount(0)

	masters := make([]MasterAddressWithAttributes, 0, totalCount)
	err = ytClient.ListNode(ctx, mastersPath, &masters, &yt.ListNodeOptions{
		Attributes: []string{"maintenance"},
	})
	if err != nil {
		return false, "", err
	}

	leaders := make([]string, 0, 1)
	followers := make([]string, 0, totalCount)
	var inactive, maintenance []string
	for _, master := range masters {
		var hydra MasterHydra
		hydraPath := mastersPath.Child(master.Address).Child(consts.MasterHydraPath)
		if err := ytClient.GetNode(ctx, hydraPath, &hydra, nil); err != nil {
			return false, "", err
		}
		name := fmt.Sprintf("[%d] %s", hydra.SelfID, master.Address)
		if !hydra.Active {
			inactive = append(inactive, name)
		}
		if master.Maintenance {
			maintenance = append(maintenance, name)
		}
		switch hydra.State {
		case MasterStateLeading:
			leaders = append(leaders, name)
		case MasterStateFollowing:
			followers = append(followers, name)
		}
	}

	var note string
	switch {
	case len(maintenance) != 0:
		note = "There is a master in maintenance"
	case len(inactive) != 0:
		note = "There is a non-active master"
	case len(leaders) != 1:
		note = "There is no single leader"
	case len(followers)+1 < int(requiredCount):
		note = "Not enough followers"
	default:
		note = "Quorum is OK"
		ok = true
	}

	msg = fmt.Sprintf("%v: leaders/followers/required/total=%d/%d/%d/%d, leaders=%v, followers=%v, inactive=%v, maintenance=%v",
		note, len(leaders), len(followers), requiredCount, totalCount, leaders, followers, inactive, maintenance)
	return ok, msg, nil
}
