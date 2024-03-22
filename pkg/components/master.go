package components

import (
	"context"
	"fmt"
	"strings"

	"go.ytsaurus.tech/yt/go/yt"

	"go.ytsaurus.tech/library/go/ptr"
	"go.ytsaurus.tech/yt/go/yson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

const (
	defaultHostAddressLabel = "kubernetes.io/hostname"
)

var (
	masterUpdatePossibleCond         = isTrue("MasterUpdatePossible")
	masterSafeModeEnabledCond        = isTrue("MasterSafeModeEnabled")
	masterSnapshotsBuildStartedCond  = isTrue("MasterSnapshotsBuildStarted")
	masterSnapshotsBuildFinishedCond = isTrue("MasterSnapshotsBuildFinished")
	masterExitReadOnlyPrepared       = isTrue("MasterExitReadOnlyPrepared")
	masterExitReadOnlyFinished       = isTrue("MasterExitReadOnlyFinished")
	masterSafeModeDisabledCond       = isTrue("MasterSafeModeDisabled")
)

type ytsaurusClientForMaster interface {
	HandlePossibilityCheck(context.Context) (bool, string, error)
	EnableSafeMode(context.Context) error
	DisableSafeMode(context.Context) error
	GetMasterMonitoringPaths(context.Context) ([]string, error)
	StartBuildMasterSnapshots(context.Context, []string) error
	AreMasterSnapshotsBuilt(context.Context, []string) (bool, error)
}

type Master struct {
	localServerComponent
	cfgen *ytconfig.Generator

	ytClient ytsaurusClientForMaster

	initJob          *InitJob
	exitReadOnlyJob  *InitJob
	adminCredentials corev1.Secret
}

func NewMaster(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, ytClient ytsaurusClientForMaster) *Master {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelMaster,
		ComponentName:  string(consts.MasterType),
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	if resource.Spec.PrimaryMasters.InstanceSpec.MonitoringPort == nil {
		resource.Spec.PrimaryMasters.InstanceSpec.MonitoringPort = ptr.Int32(consts.MasterMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.PrimaryMasters.InstanceSpec,
		"/usr/bin/ytserver-master",
		"ytserver-master.yson",
		cfgen.GetMastersStatefulSetName(),
		cfgen.GetMastersServiceName(),
		func() ([]byte, error) { return cfgen.GetMasterConfig(&resource.Spec.PrimaryMasters) },
	)

	initJob := NewInitJob(
		&l,
		ytsaurus.APIProxy(),
		ytsaurus,
		resource.Spec.ImagePullSecrets,
		"default",
		consts.ClientConfigFileName,
		resource.Spec.CoreImage,
		cfgen.GetNativeClientConfig)

	exitReadOnlyJob := NewInitJob(
		&l,
		ytsaurus.APIProxy(),
		ytsaurus,
		resource.Spec.ImagePullSecrets,
		"exit-read-only",
		consts.ClientConfigFileName,
		resource.Spec.CoreImage,
		cfgen.GetNativeClientConfig,
	)

	return &Master{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		ytClient:             ytClient,
		initJob:              initJob,
		exitReadOnlyJob:      exitReadOnlyJob,
	}
}

func (m *Master) GetType() consts.ComponentType { return consts.MasterType }

func (m *Master) IsUpdatable() bool {
	return true
}

func (m *Master) Fetch(ctx context.Context) error {
	if m.ytsaurus.GetResource().Spec.AdminCredentials != nil {
		err := m.ytsaurus.APIProxy().FetchObject(
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
		m.exitReadOnlyJob,
	)
}

func (m *Master) initAdminUser() string {
	adminLogin, adminPassword := consts.DefaultAdminLogin, consts.DefaultAdminPassword
	adminToken := consts.DefaultAdminPassword

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

	commands := createUserCommand(adminLogin, adminPassword, adminToken, true)
	return RunIfNonexistent(fmt.Sprintf("//sys/users/%s", adminLogin), commands...)
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
		commands = append(commands, fmt.Sprintf("/usr/bin/yt get //sys/media/%s/@name || /usr/bin/yt create medium --attr '%s'", medium.Name, string(attr)))
	}
	return strings.Join(commands, "\n")
}

func (m *Master) initGroups() string {
	commands := []string{
		"/usr/bin/yt create group --attr '{name=admins}' --ignore-existing",
	}
	return strings.Join(commands, "\n")
}

func (m *Master) initSchemaACLs() string {
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
		commands = append(commands, SetPathAcl(fmt.Sprintf("//sys/schemas/%s", objectType), []yt.ACE{
			userReadACE,
			adminACE,
		}))
	}
	// COMPAT(achulkov2): Drop the first command after `medium` is obsolete in all major versions.
	commands = append(commands, fmt.Sprintf("%s || %s",
		SetPathAcl("//sys/schemas/medium", []yt.ACE{
			userReadACE,
			adminACE,
		}),
		SetPathAcl("//sys/schemas/domestic_medium", []yt.ACE{
			userReadCreateACE,
			adminACE,
		})))

	// Users can create pools, pool trees, accounts and access control objects given the right circumstances and permissions.
	for _, objectType := range []string{"account", "scheduler_pool", "scheduler_pool_tree", "access_control_object"} {
		commands = append(commands, SetPathAcl(fmt.Sprintf("//sys/schemas/%s", objectType), []yt.ACE{
			userReadCreateACE,
			adminACE,
		}))
	}

	// Users can write account_resource_usage_lease objects.
	commands = append(commands, SetPathAcl("//sys/schemas/account_resource_usage_lease", []yt.ACE{
		userReadWriteCreateACE,
		adminACE,
	}))

	return strings.Join(commands, "\n")
}

func (m *Master) createInitScript() string {
	clusterConnection, err := m.cfgen.GetClusterConnection()
	if err != nil {
		panic(err)
	}

	script := []string{
		initJobWithNativeDriverPrologue(),
		m.initGroups(),
		RunIfExists("//sys/@provision_lock", m.initSchemaACLs()),
		"/usr/bin/yt create scheduler_pool_tree --attributes '{name=default; config={nodes_filter=\"\"}}' --ignore-existing",
		SetWithIgnoreExisting("//sys/pool_trees/@default_tree", "default"),
		RunIfNonexistent("//sys/pools", "/usr/bin/yt link //sys/pool_trees/default //sys/pools"),
		"/usr/bin/yt create scheduler_pool --attributes '{name=research; pool_tree=default}' --ignore-existing",
		"/usr/bin/yt create map_node //home --ignore-existing",
		RunIfExists("//sys/@provision_lock", fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection '%s'", string(clusterConnection))),
		SetWithIgnoreExisting("//sys/controller_agents/config/operation_options/spec_template", "'{enable_partitioned_data_balancing=%false}' -r"),
		m.initAdminUser(),
		m.initMedia(),
		"/usr/bin/yt remove //sys/@provision_lock -f",
	}

	return strings.Join(script, "\n")
}

func (m *Master) createExitReadOnlyScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		"export YT_LOG_LEVEL=DEBUG",
		// COMPAT(l0kix2): remove || part when the compatibility with 23.1 and older is dropped.
		`[[ "$YTSAURUS_VERSION" < "23.2" ]] && echo "master_exit_read_only is supported since 23.2, nothing to do" && exit 0`,
		"/usr/bin/yt execute master_exit_read_only '{}'",
	}

	return strings.Join(script, "\n")
}

func (m *Master) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(m.ytsaurus.GetClusterState()) && m.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), err
	}

	if m.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if m.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForMasterExitReadOnly {
			st, err := m.exitReadOnly(ctx, dry)
			return *st, err
		}
		if status, err := handleUpdatingClusterState(ctx, m.ytsaurus, m, &m.localComponent, m.server, dry); status != nil {
			return *status, err
		}
	}

	if m.NeedSync() {
		if !dry {
			err = m.doServerSync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !m.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	if !dry {
		m.initJob.SetInitScript(m.createInitScript())
	}

	return m.initJob.Sync(ctx, dry)
}

func (m *Master) Status(ctx context.Context) (ComponentStatus, error) {
	if err := m.Fetch(ctx); err != nil {
		return ComponentStatus{}, fmt.Errorf("failed to fetch component %s: %w", m.GetName(), err)
	}

	st, msg, err := m.getFlow().Status(ctx, m.condManager)
	return ComponentStatus{
		SyncStatus: st,
		Message:    msg,
	}, err
}

func (m *Master) StatusOld(ctx context.Context) ComponentStatus {
	status, err := m.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (m *Master) Sync(ctx context.Context) error {
	_, err := m.getFlow().Run(ctx, m.condManager)
	return err
}

func (m *Master) getFlow() Step {
	buildStartedCond := buildStarted(m.GetName())
	builtFinishedCond := buildFinished(m.GetName())
	initCond := initializationFinished(m.GetName())
	updateRequiredCond := updateRequired(m.GetName())
	rebuildStartedCond := rebuildStarted(m.GetName())
	rebuildFinishedCond := rebuildFinished(m.GetName())

	return StepComposite{
		Steps: []Step{
			StepRun{
				Name:               StepStartBuild,
				RunIfCondition:     not(buildStartedCond),
				RunFunc:            m.server.Sync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					diff, err := m.server.hasDiff(ctx)
					return !diff, err
				},
			},
			StepCheck{
				Name:               StepInitFinished,
				RunIfCondition:     not(initCond),
				OnSuccessCondition: initCond,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					m.initJob.SetInitScript(m.createInitScript())
					st, err := m.initJob.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update (master exit read only, safe mode, etc.).
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					diff, err := m.server.hasDiff(ctx)
					if err != nil {
						return "", "", err
					}
					if diff {
						if err = m.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					// Sync either if diff or is condition set
					// in the middle of update there will be no diff, so we need a condition.
					if diff || m.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				OnSuccessFunc:      m.cleanupAfterUpdate,
				Steps: []Step{
					StepCheck{
						Name:           "UpdatePossibleCheck",
						RunIfCondition: not(masterUpdatePossibleCond),
						StatusFunc: func(ctx context.Context) (SyncStatus, string, error) {
							possible, msg, err := m.ytClient.HandlePossibilityCheck(ctx)
							st := SyncStatusReady
							if !possible {
								st = SyncStatusBlocked
							}
							return st, msg, err
						},
						OnSuccessCondition: masterUpdatePossibleCond,
					},
					StepRun{
						Name:               "EnableSafeMode",
						RunIfCondition:     not(masterSafeModeEnabledCond),
						OnSuccessCondition: masterSafeModeEnabledCond,
						RunFunc:            m.ytClient.EnableSafeMode,
					},
					StepRun{
						Name:               "BuildSnapshots",
						RunIfCondition:     not(masterSnapshotsBuildStartedCond),
						OnSuccessCondition: masterSnapshotsBuildStartedCond,
						RunFunc: func(ctx context.Context) error {
							monitoringPaths, err := m.ytClient.GetMasterMonitoringPaths(ctx)
							if err != nil {
								return err
							}
							if err = m.storeMasterMonitoringPaths(ctx, monitoringPaths); err != nil {
								return err
							}
							return m.ytClient.StartBuildMasterSnapshots(ctx, monitoringPaths)
						},
					},
					StepCheck{
						Name:               "CheckSnapshotsBuild",
						RunIfCondition:     not(masterSnapshotsBuildFinishedCond),
						OnSuccessCondition: masterSnapshotsBuildFinishedCond,
						RunFunc: func(ctx context.Context) (ok bool, err error) {
							paths := m.getStoredMasterMonitoringPaths()
							return m.ytClient.AreMasterSnapshotsBuilt(ctx, paths)
						},
					},
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            m.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc: func(ctx context.Context) (ok bool, err error) {
							diff, err := m.server.hasDiff(ctx)
							return !diff, err
						},
					},
					StepRun{
						Name:               "StartMasterExitReadOnly",
						RunIfCondition:     not(masterExitReadOnlyPrepared),
						OnSuccessCondition: masterExitReadOnlyPrepared,
						RunFunc: func(ctx context.Context) error {
							return m.exitReadOnlyJob.prepareRestart(ctx, false)
						},
					},
					StepCheck{
						Name:               "WaitMasterExitsReadOnly",
						RunIfCondition:     not(masterExitReadOnlyFinished),
						OnSuccessCondition: masterExitReadOnlyFinished,
						RunFunc: func(ctx context.Context) (ok bool, err error) {
							m.exitReadOnlyJob.SetInitScript(m.createInitScript())
							st, err := m.exitReadOnlyJob.Sync(ctx, false)
							return st.SyncStatus == SyncStatusReady, err
						},
					},
					StepRun{
						Name:               "DisableSafeMode",
						RunIfCondition:     not(masterSafeModeDisabledCond),
						OnSuccessCondition: masterSafeModeDisabledCond,
						RunFunc:            m.ytClient.DisableSafeMode,
					},
				},
			},
		},
	}
}

func (m *Master) cleanupAfterUpdate(ctx context.Context) error {
	for _, cond := range m.getConditionsSetByUpdate() {
		if err := m.condManager.SetCond(ctx, not(cond)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Master) getConditionsSetByUpdate() []Condition {
	var result []Condition
	conds := []Condition{
		masterUpdatePossibleCond,
		masterSafeModeEnabledCond,
		masterSnapshotsBuildStartedCond,
		masterSnapshotsBuildFinishedCond,
		rebuildStarted(m.GetName()),
		rebuildFinished(m.GetName()),
		masterExitReadOnlyPrepared,
		masterExitReadOnlyFinished,
		masterSafeModeDisabledCond,
	}
	for _, cond := range conds {
		if m.condManager.IsSatisfied(cond) {
			result = append(result, cond)
		}
	}
	return result
}

func (m *Master) doServerSync(ctx context.Context) error {
	statefulSet := m.server.buildStatefulSet()
	primaryMastersSpec := m.ytsaurus.GetResource().Spec.PrimaryMasters
	if len(primaryMastersSpec.HostAddresses) != 0 {
		AddAffinity(statefulSet, m.getHostAddressLabel(), primaryMastersSpec.HostAddresses)
	}
	return m.server.Sync(ctx)
}

func (m *Master) getHostAddressLabel() string {
	primaryMastersSpec := m.ytsaurus.GetResource().Spec.PrimaryMasters
	if primaryMastersSpec.HostAddressLabel != "" {
		return primaryMastersSpec.HostAddressLabel
	}
	return defaultHostAddressLabel
}

func (m *Master) exitReadOnly(ctx context.Context, dry bool) (*ComponentStatus, error) {
	if !m.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionMasterExitReadOnlyPrepared) {
		if !m.exitReadOnlyJob.isRestartPrepared() {
			if err := m.exitReadOnlyJob.prepareRestart(ctx, dry); err != nil {
				return ptr.T(SimpleStatus(SyncStatusUpdating)), err
			}
		}

		if !dry {
			m.setMasterReadOnlyExitPrepared(ctx, metav1.ConditionTrue)
		}
		return ptr.T(SimpleStatus(SyncStatusUpdating)), nil
	}

	if !m.exitReadOnlyJob.IsCompleted() {
		if !dry {
			m.exitReadOnlyJob.SetInitScript(m.createExitReadOnlyScript())
		}
		status, err := m.exitReadOnlyJob.Sync(ctx, dry)
		return &status, err
	}

	if !dry {
		m.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionMasterExitedReadOnly,
			Status:  metav1.ConditionTrue,
			Reason:  "MasterExitedReadOnly",
			Message: "Masters exited read-only state",
		})
		m.setMasterReadOnlyExitPrepared(ctx, metav1.ConditionFalse)
	}
	return ptr.T(SimpleStatus(SyncStatusUpdating)), nil
}

func (m *Master) setMasterReadOnlyExitPrepared(ctx context.Context, status metav1.ConditionStatus) {
	m.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionMasterExitReadOnlyPrepared,
		Status:  status,
		Reason:  "MasterExitReadOnlyPrepared",
		Message: "Masters are ready to exit read-only state",
	})
}

func (m *Master) storeMasterMonitoringPaths(ctx context.Context, paths []string) error {
	return m.stateManager.SetMasterMonitoringPaths(ctx, paths)
}

func (m *Master) getStoredMasterMonitoringPaths() []string {
	return m.stateManager.GetMasterMonitoringPaths()
}
