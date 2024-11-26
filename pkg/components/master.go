package components

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"

	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	defaultHostAddressLabel = "kubernetes.io/hostname"
)

type Master struct {
	localServerComponent
	cfgen *ytconfig.Generator

	initJob          *InitJob
	exitReadOnlyJob  *InitJob
	adminCredentials corev1.Secret
}

func NewMaster(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *Master {
	l := cfgen.GetComponentLabeller(consts.MasterType, "")

	resource := ytsaurus.GetResource()
	if resource.Spec.PrimaryMasters.InstanceSpec.MonitoringPort == nil {
		resource.Spec.PrimaryMasters.InstanceSpec.MonitoringPort = ptr.To(int32(consts.MasterMonitoringPort))
	}

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.PrimaryMasters.InstanceSpec,
		"/usr/bin/ytserver-master",
		"ytserver-master.yson",
		func() ([]byte, error) { return cfgen.GetMasterConfig(&resource.Spec.PrimaryMasters) },
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.MasterRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	initJob := NewInitJob(
		l,
		ytsaurus.APIProxy(),
		ytsaurus,
		resource.Spec.ImagePullSecrets,
		"default",
		consts.ClientConfigFileName,
		getImageWithDefault(resource.Spec.PrimaryMasters.InstanceSpec.Image, resource.Spec.CoreImage),
		cfgen.GetNativeClientConfig,
		getTolerationsWithDefault(resource.Spec.PrimaryMasters.Tolerations, resource.Spec.Tolerations),
		getNodeSelectorWithDefault(resource.Spec.PrimaryMasters.NodeSelector, resource.Spec.NodeSelector),
	)

	exitReadOnlyJob := NewInitJob(
		l,
		ytsaurus.APIProxy(),
		ytsaurus,
		resource.Spec.ImagePullSecrets,
		"exit-read-only",
		consts.ClientConfigFileName,
		getImageWithDefault(resource.Spec.PrimaryMasters.InstanceSpec.Image, resource.Spec.CoreImage),
		cfgen.GetNativeClientConfig,
		getTolerationsWithDefault(resource.Spec.PrimaryMasters.Tolerations, resource.Spec.Tolerations),
		getNodeSelectorWithDefault(resource.Spec.PrimaryMasters.NodeSelector, resource.Spec.NodeSelector),
	)

	return &Master{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		initJob:              initJob,
		exitReadOnlyJob:      exitReadOnlyJob,
	}
}

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
		// COMPAT(gritukan): Remove "medium" after some time.
		commands = append(commands, fmt.Sprintf("/usr/bin/yt get //sys/media/%s/@name || /usr/bin/yt create domestic_medium --attr '%s' || /usr/bin/yt create medium --attr '%s'", medium.Name, string(attr), string(attr)))
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
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
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
	return m.doSync(ctx, true)
}

func (m *Master) Sync(ctx context.Context) error {
	_, err := m.doSync(ctx, false)
	return err
}

func (m *Master) doServerSync(ctx context.Context) error {
	statefulSet := m.server.buildStatefulSet()
	podSpec := &statefulSet.Spec.Template.Spec
	primaryMastersSpec := m.ytsaurus.GetResource().Spec.PrimaryMasters

	if err := AddSidecarsToPodSpec(primaryMastersSpec.Sidecars, podSpec); err != nil {
		return err
	}

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
				return ptr.To(SimpleStatus(SyncStatusUpdating)), err
			}
		}

		if !dry {
			m.setMasterReadOnlyExitPrepared(ctx, metav1.ConditionTrue)
		}
		return ptr.To(SimpleStatus(SyncStatusUpdating)), nil
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
	return ptr.To(SimpleStatus(SyncStatusUpdating)), nil
}

func (m *Master) setMasterReadOnlyExitPrepared(ctx context.Context, status metav1.ConditionStatus) {
	m.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionMasterExitReadOnlyPrepared,
		Status:  status,
		Reason:  "MasterExitReadOnlyPrepared",
		Message: "Masters are ready to exit read-only state",
	})
}
