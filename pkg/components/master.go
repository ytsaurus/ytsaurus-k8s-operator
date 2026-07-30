package components

import (
	"context"
	"fmt"
	"slices"
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

	secondaryMasters []*Master
}

func buildMasterOptions(mastersSpec *ytv1.MastersSpec) []Option {
	options := []Option{
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.MasterRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	}

	if mastersSpec.HydraPersistenceUploader != nil && mastersSpec.HydraPersistenceUploader.Image != nil {
		options = append(options, WithSidecarImage(
			consts.HydraPersistenceUploaderContainerName,
			*mastersSpec.HydraPersistenceUploader.Image,
		))
	}

	// Masters keep a per-component timbertruck override; serverImpl handles the sidecar generically.
	options = append(options, WithTimbertruck(mastersSpec.Timbertruck))

	return options
}

func NewMaster(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	mastersSpec *ytv1.MastersSpec,
	secondaryMasters []*Master,
) *Master {
	l := cfgen.GetMasterLabeller(mastersSpec.CellTag)

	srv := newServer(
		l,
		ytsaurus,
		&mastersSpec.InstanceSpec,
		"/usr/bin/ytserver-master",
		[]ConfigGenerator{
			ServerConfigGenerator(l, func() ([]byte, error) {
				return cfgen.GetMasterConfig(mastersSpec)
			}),
			{
				FileName:  consts.ClientConfigFileName,
				Format:    ConfigFormatYson,
				Generator: cfgen.GetNativeClientConfig,
			},
		},
		cfgen.GetTimbertruckConfig,
		consts.MasterMonitoringPort,
		buildMasterOptions(mastersSpec)...,
	)

	var uploaderSecret *resources.StringSecret
	if mastersSpec.HydraPersistenceUploader != nil {
		uploaderSecret = resources.NewStringSecret(buildUserCredentialsSecretname(consts.HydraPersistenceUploaderUserName), l, ytsaurus)
	}

	master := &Master{
		serverComponent:  newLocalServerComponent(l, ytsaurus, srv),
		mastersSpec:      mastersSpec,
		cfgen:            cfgen,
		uploaderSecret:   uploaderSecret,
		secondaryMasters: secondaryMasters,
	}

	// Only for primary master.
	if l.InstanceGroup == "" {
		master.initJob = NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"default",
			&mastersSpec.InstanceSpec,
			YsonConfigGenerator(consts.ClientConfigFileName, cfgen.GetNativeClientConfig),
			TextConfigGenerator(consts.MasterCellsInitializationScriptName, master.scriptMasterCellsInitialization),
			TextConfigGenerator(consts.ClusterInitializationScriptName, master.scriptInitialization),
			TextConfigGenerator(consts.MasterEnterReadOnlyScriptName, master.scriptEnterReadOnly),
			TextConfigGenerator(consts.MasterExitReadOnlyScriptName, master.scriptExitReadOnly),
			TextConfigGenerator(consts.MasterCellsPreparationScriptName, master.scriptMasterCellsPreparation),
			TextConfigGenerator(consts.MasterCellsWaitRegistrationScriptName, master.scriptWaitingMasterCellsRegistation),
			TextConfigGenerator(consts.MasterCellsSettlementScriptName, master.scriptMasterCellsSettlement),
			TextConfigGenerator(consts.MasterCellsCompletionScriptName, master.scriptMasterCellsCompletion),
		)
	}

	return master
}

func (m *Master) IsPrimary() bool {
	return m.labeller.InstanceGroup == ""
}

func (m *Master) IsUnregistered() bool {
	maintenance := m.ytsaurus.GetResource().Status.UpdateStatus.MasterCellsMaintenance
	return slices.ContainsFunc(maintenance, func(info ytv1.MasterCellMaintenanceInfo) bool {
		return info.CellTag == m.mastersSpec.CellTag && info.Unregistered
	})
}

func (m *Master) SetUnregistered(unregistered bool) {
	maintenance := m.ytsaurus.GetResource().Status.UpdateStatus.MasterCellsMaintenance
	maintenance = slices.DeleteFunc(maintenance, func(info ytv1.MasterCellMaintenanceInfo) bool {
		return info.CellTag == m.mastersSpec.CellTag
	})
	if unregistered {
		maintenance = append(maintenance, ytv1.MasterCellMaintenanceInfo{
			CellTag:      m.mastersSpec.CellTag,
			Unregistered: true,
		})
	}
	m.ytsaurus.GetResource().Status.UpdateStatus.MasterCellsMaintenance = maintenance
	m.owner.RecordNormal("MasterCell", fmt.Sprintf("Maintenance %+v", maintenance))
}

func (m *Master) GetCypressPath() ypath.Path {
	if m.IsPrimary() {
		return consts.PrimaryMastersPath
	} else {
		return ypath.Path(consts.SecondaryMastersPath).Child(m.labeller.InstanceGroup)
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

func (m *Master) ArePodsReady(ctx context.Context) (ComponentStatus, error) {
	return arePodsReady(ctx, m.server, m.labeller, []string{consts.YTServerContainerName})
}

func (m *Master) getAdminCredentials() (exists bool, adminLogin string, adminPassword string, adminToken string) {
	if m.adminCredentials.Name != "" {
		adminLogin = string(m.adminCredentials.Data[consts.AdminLoginSecretKey])
		adminPassword = string(m.adminCredentials.Data[consts.AdminPasswordSecretKey])
		adminToken = string(m.adminCredentials.Data[consts.AdminTokenSecretKey])
		exists = adminLogin != "" && adminPassword != "" && adminToken != ""
	}
	return exists, adminLogin, adminPassword, adminToken
}

func (m *Master) initAdminUser() []string {
	exists, adminLogin, adminPassword, adminToken := m.getAdminCredentials()
	if !exists {
		return nil
	}
	return createUserCommand(adminLogin, adminPassword, adminToken, true)
}

func (m *Master) initUploaderUser() (string, error) {
	if m.uploaderSecret == nil {
		return "", nil
	}

	login := consts.HydraPersistenceUploaderUserName
	token, _ := m.uploaderSecret.GetValue(consts.TokenSecretKey)
	commands := []string{
		strings.Join(createUserCommand(login, "", token, false), "\n"),
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

func (m *Master) scriptMasterCellDescriptors() ([]string, error) {
	cellDescriptors := ytconfig.GetMasterCellDescriptors(m.mastersSpec, m.ytsaurus.GetResource().Spec.SecondaryMasters)
	config, err := yson.MarshalFormat(cellDescriptors, yson.FormatPretty)
	if err != nil {
		return nil, err
	}
	return []string{
		RunIfNonexistent("//sys/@provision_lock", `exit 1`),
		`test "$(/usr/bin/yt get //sys/@dynamically_propagated_masters_cell_tags)" = '[]'`,
		fmt.Sprintf("/usr/bin/yt set %s '%s'", consts.MasterCellDescriptorsPath, string(config)),
		`/usr/bin/yt set //sys/@config/multicell_manager/remove_secondary_cell_default_roles %true`, // NOTE: This is default since 25.3
	}, nil
}

func (m *Master) scriptMasterCellsInitialization() ([]string, error) {
	return JoinTextGenerators(
		PlainTextGenerator(initJobWithNativeDriverPrologue()),
		m.scriptWaitingMasterCellsRegistation,
		m.scriptMasterCellDescriptors,
		PlainTextGenerator(masterEnterReadOnly),
	)()
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
		masterExitReadOnly,
		m.initGroups(),
		RunIfExists("//sys/@provision_lock", initSchemaACLsCommands),
		"/usr/bin/yt create scheduler_pool_tree --attributes '{name=default; config={nodes_filter=\"\"}}' --ignore-existing",
		SetWithIgnoreExisting("//sys/pool_trees/@default_tree", "default"),
		RunIfNonexistent("//sys/pools", "/usr/bin/yt link //sys/pool_trees/default //sys/pools"),
		RunIfNonexistent("//sys/pool_trees/default/research", "/usr/bin/yt create scheduler_pool --attributes '{name=research; pool_tree=default}'"),
		"/usr/bin/yt create map_node //home --ignore-existing",
		RunIfExists("//sys/@provision_lock", fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection '%s'", string(connConfig))),
		RunIfExists("//sys/@provision_lock", fmt.Sprintf("/usr/bin/yt set //sys/@cluster_name '%s'", clusterConn.ClusterName)),
		m.initMedia(),
	}

	initCommands = append(initCommands, m.initAdminUser()...)

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

func (m *Master) scriptDumpCellOrchids() ([]string, error) {
	var script []string
	for _, addreess := range m.cfgen.GetMasterCellAddresses(m.mastersSpec) {
		path := m.GetCypressPath().Child(addreess)
		script = append(script,
			fmt.Sprintf(`/usr/bin/yt get '%s' || :`, path.Child(consts.MasterElectionPath)),
			fmt.Sprintf(`/usr/bin/yt get '%s' || :`, path.Child(consts.MasterHydraPath)))
	}
	return script, nil
}

func (m *Master) scriptDumpAllOrchids() []TextGeneratorFunc {
	orchids := []TextGeneratorFunc{m.scriptDumpCellOrchids}
	for _, secondary := range m.secondaryMasters {
		orchids = append(orchids, secondary.scriptDumpCellOrchids)
	}
	return orchids
}

const (
	masterEnterReadOnly = `YT_LOG_LEVEL=Debug /usr/bin/yt execute build_master_snapshots '{ set_read_only=%true; wait_for_snapshot_completion=%true; retry=%true; }'`
	masterExitReadOnly  = `YT_LOG_LEVEL=Debug /usr/bin/yt execute master_exit_read_only '{}'`
)

func (m *Master) scriptEnterReadOnly() ([]string, error) {
	return JoinTextGenerators(
		PlainTextGenerator(initJobWithNativeDriverPrologue()),
		JoinTextGenerators(m.scriptDumpAllOrchids()...),
		PlainTextGenerator(masterEnterReadOnly),
	)()
}

func (m *Master) scriptExitReadOnly() ([]string, error) {
	return JoinTextGenerators(
		PlainTextGenerator(initJobWithNativeDriverPrologue()),
		JoinTextGenerators(m.scriptDumpAllOrchids()...),
		PlainTextGenerator(masterExitReadOnly),
	)()
}

func (m *Master) scriptMasterCellsPreparation() ([]string, error) {
	return []string{
		initJobWithNativeDriverPrologue(),
		`/usr/bin/yt set //sys/@provision_lock %true`,
		`/usr/bin/yt set //sys/@config/chunk_manager/enable_chunk_refresh %false`,
		`/usr/bin/yt set //sys/@config/chunk_manager/enable_chunk_requisition_update %false`,
		`/usr/bin/yt set //sys/@config/multicell_manager/testing/allow_master_cell_with_empty_role %true`,
		`/usr/bin/yt set //sys/@config/multicell_manager/remove_secondary_cell_default_roles %true`,
	}, nil
}

func (m *Master) scriptMasterCellsCompletion() ([]string, error) {
	return []string{
		initJobWithNativeDriverPrologue(),
		`/usr/bin/yt set //sys/@config/chunk_manager/enable_chunk_refresh %true`,
		`/usr/bin/yt set //sys/@config/chunk_manager/enable_chunk_requisition_update %true`,
		"/usr/bin/yt remove //sys/@provision_lock -f",
	}, nil
}

func (m *Master) scriptWaitingMasterCellsRegistation() ([]string, error) {
	commands := []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf(
			`while test "$(/usr/bin/yt get --format json //sys/@registered_master_cell_tags | jq -c sort)" != '%s'; do sleep 1; done`,
			m.cfgen.GetMasterCellTagsAsSortedJSON(true),
		),
	}
	if false {
		// //sys/secondary_masters is filled by world initialization which happens every 5 minutes.
		testCell := func(spec *ytv1.MastersSpec, path, tag string) {
			for _, address := range m.cfgen.GetMasterCellAddresses(spec) {
				commands = append(commands, fmt.Sprintf(`test "$(yt get %s%s/%s/%s/active)" = %%true`, path, tag, address, consts.MasterHydraPath))
			}
		}
		testCell(m.mastersSpec, "//sys/primary_masters", "")
		for _, secondary := range m.secondaryMasters {
			testCell(secondary.mastersSpec, "//sys/secondary_masters/", secondary.labeller.InstanceGroup)
		}
	}
	return commands, nil
}

func (m *Master) scriptMasterCellsSettlement() ([]string, error) {
	return JoinTextGenerators(
		PlainTextGenerator(initJobWithNativeDriverPrologue()),
		m.scriptMasterCellDescriptors,
	)()
}

func (m *Master) NeedUpdate() ComponentStatus {
	// NOTE: See master maintenance update flow.
	if m.ytsaurus.GetClusterMaintenance().Shutdown == ytv1.ClusterShutdownExceptMasters {
		switch {
		case m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsPreparation):
			return ComponentStatusNeedUpdate("Master cells preparation")
		case m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsRegistration):
			return ComponentStatusNeedUpdate("Master cells registration")
		case m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsCompletion):
			return ComponentStatusNeedUpdate("Master cells completion")
		}
	}
	if !m.IsPrimary() && m.IsUnregistered() && !m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsRegistration) {
		return ComponentStatusBlocked("Secondary master cell %v cannot register yet", m.labeller.InstanceGroup)
	}
	return m.server.needUpdate()
}

//nolint:cyclop,nestif //this is complex function
func (m *Master) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if m.ytsaurus.IsUpdating() {
		if !IsUpdatingComponent(m.ytsaurus, m) {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}

		updateState := m.ytsaurus.GetUpdateState()

		if m.IsPrimary() {
			switch updateState {
			case ytv1.UpdateStateWaitingForSnapshots:
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.MasterEnterReadOnlyScriptName, func() {
					m.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
						Type:    consts.ConditionSnaphotsSaved,
						Status:  metav1.ConditionTrue,
						Reason:  "Update",
						Message: "Master read-only snapshots were built",
					})
				})
			case ytv1.UpdateStateWaitingForMasterEnterReadOnly, ytv1.UpdateStateWaitingForMasterCellsEnterReadOnly:
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.MasterEnterReadOnlyScriptName, nil)
			case ytv1.UpdateStateWaitingForMasterExitReadOnly, ytv1.UpdateStateWaitingForMasterCellsExitReadOnly:
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.MasterExitReadOnlyScriptName, nil)
			case ytv1.UpdateStateWaitingForSidecarsInitialize:
				// TODO: Split into separate script.
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.ClusterInitializationScriptName, nil)
			case ytv1.UpdateStateWaitingForMasterCellsPreparation:
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.MasterCellsPreparationScriptName, nil)
			case ytv1.UpdateStateWaitingForMasterCellsAcception:
				if !dry {
					// Finish master cells preparation.
					m.owner.RemoveStatusCondition(consts.ConditionMasterCellsPreparation)
					// Open master cells registration.
					m.owner.SetStatusCondition(metav1.Condition{
						Type:    consts.ConditionMasterCellsRegistration,
						Status:  metav1.ConditionTrue,
						Reason:  consts.PhaseCellRegistration,
						Message: "Master cells registration is open",
					})
					m.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
						Type:    m.ytsaurus.GetUpdateStateCompleteCondition(updateState),
						Status:  metav1.ConditionTrue,
						Reason:  "Complete",
						Message: "Done",
					})
				}
				return ComponentStatusWaitingFor("opening master cell registration"), nil
			case ytv1.UpdateStateWaitingForMasterCellsRegistration:
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.MasterCellsWaitRegistrationScriptName, func() {
					for _, secondary := range m.secondaryMasters {
						if secondary.mastersSpec.InstanceCount > 0 && secondary.IsUnregistered() {
							secondary.SetUnregistered(false)
						}
					}
					// Close master cells registration.
					m.owner.RemoveStatusCondition(consts.ConditionMasterCellsRegistration)
				})
			case ytv1.UpdateStateWaitingForMasterCellsSettlement:
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.MasterCellsSettlementScriptName, func() {
					m.owner.SetStatusCondition(metav1.Condition{
						Type:    consts.ConditionMasterCellsInitialized,
						Status:  metav1.ConditionTrue,
						Reason:  consts.PhaseCellsRolesAssignment,
						Message: "Master cells roles assignment complete",
					})
					// Trigger master cells completion pass.
					m.owner.SetStatusCondition(metav1.Condition{
						Type:    consts.ConditionMasterCellsCompletion,
						Status:  metav1.ConditionTrue,
						Reason:  consts.PhaseCellRegistration,
						Message: "Master cells completion is pending",
					})
				})
			case ytv1.UpdateStateWaitingForMasterCellsCompletion:
				return m.initJob.RunUpdateScript(ctx, dry, m.ytsaurus, updateState, consts.MasterCellsCompletionScriptName, func() {
					// Finish master cells completion pass.
					m.owner.RemoveStatusCondition(consts.ConditionMasterCellsCompletion)
				})
			}
		}

		switch updateState {
		case ytv1.UpdateStateWaitingForPodsRemoval:
			// TODO: Cleanup, add separate update states for strategies.
			switch getComponentUpdateStrategy(m.ytsaurus, consts.MasterType, m.GetShortName()) {
			case ytv1.ComponentUpdateModeTypeOnDelete:
				if status, err := handleOnDeleteUpdatingClusterState(ctx, m.ytsaurus, m, &m.component, m.server, dry); status != nil {
					return *status, err
				}
			default:
				if status, err := handleBulkUpdatingClusterState(ctx, m.ytsaurus, m, &m.component, m.server, dry); status != nil {
					return *status, err
				}
			}
		case ytv1.UpdateStateWaitingForMasterReady:
			// Masters are upgraded first at separate update state.
			// Sync server and create pods below.
		case ytv1.UpdateStateWaitingForMasterCellsRegistration:
			// Sync secondary cells masters when to register them.
		default:
			return ComponentStatusReadyAfter("No actions required for this update state"), nil
		}
	}

	if m.uploaderSecret != nil && m.uploaderSecret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			token := m.cfgen.GenerateToken()
			s := m.uploaderSecret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: token,
			}
			err = m.uploaderSecret.Sync(ctx)
		}
		return ComponentStatusWaitingFor(m.uploaderSecret.Name()), err
	}

	if !m.ytsaurus.IsInitializing() {
		initiated := m.mastersSpec.InstanceCount > 0
		unregistered := m.IsUnregistered()
		if !m.IsPrimary() && !initiated && !unregistered {
			if !dry {
				m.SetUnregistered(true)
			}
			return ComponentStatusWaitingFor("marking secondary master cell unregistered"), nil
		}
		if !m.IsPrimary() && initiated && unregistered {
			preparation := m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsPreparation)
			regisration := m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsRegistration)
			if !regisration && !preparation {
				if !dry {
					m.owner.SetStatusCondition(metav1.Condition{
						Type:    consts.ConditionMasterCellsPreparation,
						Status:  metav1.ConditionTrue,
						Reason:  consts.PhaseClusterReconfiguration,
						Message: "Secondary master cells registration is pending",
					})
				}
				return ComponentStatusWaitingFor("starting secondary master cell registration"), nil
			}
			// Delay instantiation of unregistered secondary masters until primary master opens registration.
			if !regisration {
				m.server.setInstanceCount(0)
			}
		}
		maintenance := m.ytsaurus.GetClusterMaintenance()
		cellsInitialized := m.owner.GetStatusCondition(consts.ConditionMasterCellsInitialized)
		if maintenance.Shutdown == ytv1.ClusterShutdownExceptMasters && maintenance.AssignMasterCellsRoles {
			if cellsInitialized == nil || cellsInitialized.Reason != consts.PhaseCellsRolesAssignment {
				m.owner.SetStatusCondition(metav1.Condition{
					Type:    consts.ConditionMasterCellsInitialized,
					Status:  metav1.ConditionFalse,
					Reason:  consts.PhaseCellsRolesAssignment,
					Message: "Master cells roles assignment is pending",
				})
				m.owner.SetStatusCondition(metav1.Condition{
					Type:    consts.ConditionMasterCellsPreparation,
					Status:  metav1.ConditionTrue,
					Reason:  consts.PhaseClusterReconfiguration,
					Message: "Master cells roles assignment is pending",
				})
			}
		} else if cellsInitialized != nil && cellsInitialized.Reason == consts.PhaseCellsRolesAssignment && cellsInitialized.Status == metav1.ConditionTrue {
			m.owner.SetStatusCondition(metav1.Condition{
				Type:    consts.ConditionMasterCellsInitialized,
				Status:  metav1.ConditionTrue,
				Reason:  consts.PhaseCellsRolesAssigned,
				Message: "Master cells roles assigned",
			})
		}
	}

	if status, err := m.ServerSync(ctx, dry); !status.IsReady() || err != nil {
		return status, err
	}

	if status, err := m.ArePodsReady(ctx); !status.IsReady() || err != nil {
		return status, err
	}

	for _, secondaryMaster := range m.secondaryMasters {
		if status := secondaryMaster.GetStatus(); !status.IsRunning() {
			return status.Blocker(), nil
		}
	}

	if m.IsPrimary() && m.ytsaurus.IsInitializing() {
		if status, err := m.runInitPhaseJobs(ctx, dry); !status.IsReady() || err != nil {
			return status, err
		}
	}

	return ComponentStatusReady(), nil
}

func (m *Master) ServerSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	needSync := m.NeedSync()

	if m.ytsaurus.IsInitializing() && m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsInitialized) &&
		m.server.getPodAnnotation(consts.InstancePhaseAnnotationName) == consts.PhaseCellsInitialization {
		needSync = true
	}

	if needSync {
		var err error
		if !dry {
			err = m.doServerSync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	return ComponentStatusReady(), nil
}

func (m *Master) doServerSync(ctx context.Context) error {
	statefulSet := m.server.buildStatefulSet()

	podMeta := &statefulSet.Spec.Template.ObjectMeta
	metav1.SetMetaDataLabel(podMeta, consts.YTCellTagLabelName, m.labeller.GetCellName(m.mastersSpec.CellTag))
	metav1.SetMetaDataLabel(podMeta, consts.YTCellIDLabelName, m.cfgen.GetCellID(m.mastersSpec.CellTag))

	// Arm restart of masters after cell roles initialization to refresh cached cluster metadata.
	if m.ytsaurus.IsInitializing() && !m.owner.IsStatusConditionTrue(consts.ConditionMasterCellsInitialized) {
		metav1.SetMetaDataAnnotation(podMeta, consts.InstancePhaseAnnotationName, consts.PhaseCellsInitialization)
	}

	podSpec := &statefulSet.Spec.Template.Spec
	podSpec.Containers[0].Env = append(podSpec.Containers[0].Env, getNativeClientConfigEnv()...)

	if m.mastersSpec.HydraPersistenceUploader != nil && m.mastersSpec.HydraPersistenceUploader.Image != nil {
		if err := addHydraPersistenceUploaderToPodSpec(
			*m.mastersSpec.HydraPersistenceUploader.Image,
			podSpec,
			m.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
			m.uploaderSecret.Name(),
			&m.mastersSpec.InstanceSpec,
		); err != nil {
			return err
		}
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
	var patch ypatch.Patch
	if m.IsPrimary() {
		cellID := m.cfgen.GetCellID(m.mastersSpec.CellTag)
		patch = ypatch.Patch{
			ypatch.Test("/primary_master/cell_id", cellID),
			ypatch.Replace("/primary_master/addresses", &clusterConnection.PrimaryMaster.Addresses),
			ypatch.Replace("/primary_master/peers", &clusterConnection.PrimaryMaster.Peers),
			ypatch.ReplaceOrRemove("/bus_client", clusterConnection.BusClient),
		}
	} else {
		cellID := m.cfgen.GetCellID(m.mastersSpec.CellTag)
		index := slices.IndexFunc(clusterConnection.SecondaryMasters, func(cell ytconfig.MasterCell) bool { return cell.CellID == cellID })
		if index >= 0 {
			path := ypath.Path("/secondary_masters").Child(fmt.Sprintf("%v", index))
			patch = ypatch.Patch{
				ypatch.Test(path.Child("cell_id"), cellID),
				ypatch.Replace(path.Child("addresses"), &clusterConnection.SecondaryMasters[index].Addresses),
				ypatch.Replace(path.Child("peers"), &clusterConnection.SecondaryMasters[index].Peers),
			}
		}
	}
	return ypatch.PatchSet{"//sys/@cluster_connection": patch}
}

func (m *Master) getHostAddressLabel() string {
	if m.mastersSpec.HostAddressLabel != "" {
		return m.mastersSpec.HostAddressLabel
	}
	return defaultHostAddressLabel
}

func (m *Master) runInitPhaseJobs(ctx context.Context, dry bool) (ComponentStatus, error) {
	if !m.ytsaurus.IsStatusConditionTrue(consts.ConditionMasterCellsInitialized) {
		return m.initJob.RunScript(ctx, dry, consts.PhaseCellsInitialization, consts.MasterCellsInitializationScriptName, func(status *ComponentStatus) {
			m.owner.SetStatusCondition(metav1.Condition{
				Type:    consts.ConditionMasterCellsInitialized,
				Status:  metav1.ConditionTrue,
				Reason:  consts.PhaseCellsInitialization,
				Message: "Master cell roles initialized",
			})
			*status = ComponentStatusWaitingFor("cluster initialization")
		})
	}
	return m.initJob.RunScript(ctx, dry, consts.PhaseClusterInitialization, consts.ClusterInitializationScriptName, nil)
}

func addHydraPersistenceUploaderToPodSpec(hydraImage string, podSpec *corev1.PodSpec, proxy string, secretKey string, masterInstanceSpec *ytv1.InstanceSpec) error {
	masterDataMounts, err := resolveLocationMounts(masterInstanceSpec, ytv1.LocationTypeMasterSnapshots, ytv1.LocationTypeMasterChangelogs)
	if err != nil {
		return fmt.Errorf("failed to resolve mounts for hydra persistence uploader: %w", err)
	}
	for i := range masterDataMounts {
		masterDataMounts[i].ReadOnly = true
	}

	volumeMounts := []corev1.VolumeMount{
		{Name: consts.ConfigTemplateVolumeName, MountPath: consts.ConfigMountPoint, ReadOnly: true},
	}
	volumeMounts = append(volumeMounts, masterDataMounts...)
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name: "shared-binaries", MountPath: "/shared-binaries", ReadOnly: false,
	})

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
			VolumeMounts:    volumeMounts,
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
	return nil
}
