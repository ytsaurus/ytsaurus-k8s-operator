package components

import (
	"context"
	"fmt"
	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	corev1 "k8s.io/api/core/v1"
)

type master struct {
	ServerComponentBase

	initJob          *InitJob
	adminCredentials corev1.Secret
}

func NewMaster(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: consts.YTComponentLabelMaster,
		ComponentName:  "Master",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.Masters.InstanceGroup,
		"/usr/bin/ytserver-master",
		"ytserver-master.yson",
		cfgen.GetMastersStatefulSetName(),
		cfgen.GetMastersServiceName(),
		cfgen.GetMasterConfig,
	)

	initJob := NewInitJob(
		&labeller,
		apiProxy,
		"default",
		consts.ClientConfigFileName,
		cfgen.GetNativeClientConfig)

	return &master{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &labeller,
				apiProxy: apiProxy,
				cfgen:    cfgen,
			},
			server: server,
		},
		initJob: initJob,
	}
}

func (m *master) Fetch(ctx context.Context) error {
	if m.apiProxy.Ytsaurus().Spec.AdminCredentials != nil {
		err := m.apiProxy.FetchObject(
			ctx,
			m.apiProxy.Ytsaurus().Spec.AdminCredentials.Name,
			&m.adminCredentials)
		if err != nil {
			return err
		}
	}

	return resources.Fetch(ctx, []resources.Fetchable{
		m.server,
		m.initJob,
	})
}

func (m *master) initAdminUser() string {
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
	return strings.Join(commands, "\n")
}

func (m *master) createInitScript() string {
	clusterConnection, err := m.cfgen.GetClusterConnection()
	if err != nil {
		panic(err)
	}

	script := []string{
		initJobWithNativeDriverPrologue(),
		"/usr/bin/yt remove //sys/@provision_lock -f",
		"/usr/bin/yt create scheduler_pool_tree --attributes '{name=default; config={nodes_filter=\"\"}}' --ignore-existing",
		"/usr/bin/yt set //sys/pool_trees/@default_tree default",
		"/usr/bin/yt create scheduler_pool --attributes '{name=research; pool_tree=default}' --ignore-existing",
		"/usr/bin/yt create map_node //home --ignore-existing",
		fmt.Sprintf("/usr/bin/yt set //sys/@cluster_connection '%s'", string(clusterConnection)),
		m.initAdminUser(),
	}

	return strings.Join(script, "\n")
}

func (m *master) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error

	if m.apiProxy.GetClusterState() == v1.ClusterStateRunning && m.server.NeedUpdate() {
		return SyncStatusNeedUpdate, err
	}

	if m.apiProxy.GetClusterState() == v1.ClusterStateUpdating {
		if m.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForPodsRemoval {
			return SyncStatusUpdating, m.removePods(ctx, dry)
		}
	}

	if !m.server.IsInSync() {
		if !dry {
			err = m.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !m.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	if !dry {
		m.initJob.SetInitScript(m.createInitScript())
	}

	return m.initJob.Sync(ctx, dry)
}

func (m *master) Status(ctx context.Context) SyncStatus {
	status, err := m.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (m *master) Sync(ctx context.Context) error {
	_, err := m.doSync(ctx, false)
	return err
}
