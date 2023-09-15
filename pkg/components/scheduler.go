package components

import (
	"context"
	"fmt"
	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"go.ytsaurus.tech/library/go/ptr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	corev1 "k8s.io/api/core/v1"
)

type scheduler struct {
	componentBase
	server        server
	master        Component
	execNodes     []Component
	tabletNodes   []Component
	initUser      *InitJob
	initOpArchive *InitJob
	secret        *resources.StringSecret
}

func NewScheduler(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	execNodes, tabletNodes []Component) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelScheduler,
		ComponentName:  "Scheduler",
		MonitoringPort: consts.SchedulerMonitoringPort,
	}

	server := newServer(
		&l,
		ytsaurus,
		&resource.Spec.Schedulers.InstanceSpec,
		"/usr/bin/ytserver-scheduler",
		"ytserver-scheduler.yson",
		cfgen.GetSchedulerStatefulSetName(),
		cfgen.GetSchedulerServiceName(),
		cfgen.GetSchedulerConfig,
	)

	return &scheduler{
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		server:      server,
		master:      master,
		execNodes:   execNodes,
		tabletNodes: tabletNodes,
		initUser: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig),
		initOpArchive: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"op-archive",
			consts.ClientConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
	}
}

func (s *scheduler) IsUpdatable() bool {
	return true
}

func (s *scheduler) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		s.server,
		s.initOpArchive,
		s.initUser,
		s.secret,
	})
}

func (s *scheduler) Status(ctx context.Context) ComponentStatus {
	status, err := s.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (s *scheduler) Sync(ctx context.Context) error {
	_, err := s.doSync(ctx, false)
	return err
}

func (s *scheduler) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if s.ytsaurus.GetClusterState() == v1.ClusterStateRunning && s.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if s.ytsaurus.GetClusterState() == v1.ClusterStateUpdating {
		if s.ytsaurus.GetUpdateState() == v1.UpdateStateWaitingForPodsRemoval && IsUpdatingComponent(s.ytsaurus, s) {
			if !dry {
				err = removePods(ctx, s.server, &s.componentBase)
			}
			return WaitingStatus(SyncStatusUpdating, "pods removal"), err
		}
		if status, err := s.updateOpArchive(ctx, dry); status != nil {
			return *status, err
		}
	}

	if s.master.Status(ctx).SyncStatus != SyncStatusReady {
		return WaitingStatus(SyncStatusBlocked, s.master.GetName()), err
	}

	if s.execNodes == nil || len(s.execNodes) > 0 {
		for _, end := range s.execNodes {
			if end.Status(ctx).SyncStatus != SyncStatusReady {
				// It makes no sense to start scheduler without exec nodes.
				return WaitingStatus(SyncStatusBlocked, end.GetName()), err
			}
		}
	}

	if s.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			secretSpec := s.secret.Build()
			secretSpec.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = s.secret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, s.secret.Name()), err
	}

	if s.server.needSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = s.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !s.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	if !s.needOpArchiveInit() {
		// Don't initialize operations archive.
		return SimpleStatus(SyncStatusReady), err
	}

	if !dry {
		s.initUser.SetInitScript(s.createInitScript())
	}

	status, err := s.initUser.Sync(ctx, dry)
	if status.SyncStatus != SyncStatusReady {
		return status, err
	}

	for _, tnd := range s.tabletNodes {
		if tnd.Status(ctx).SyncStatus != SyncStatusReady {
			// Wait for tablet nodes to proceed with operations archive init.
			return WaitingStatus(SyncStatusBlocked, tnd.GetName()), err
		}
	}

	if !dry {
		s.prepareInitOperationArchive()
	}
	return s.initOpArchive.Sync(ctx, dry)
}

func (s *scheduler) updateOpArchive(ctx context.Context, dry bool) (*ComponentStatus, error) {
	var err error
	switch s.ytsaurus.GetUpdateState() {
	case v1.UpdateStateWaitingForOpArchiveUpdatingPrepare:
		if !s.needOpArchiveInit() {
			s.setConditionNotNecessaryToUpdateOpArchive()
			return ptr.T(SimpleStatus(SyncStatusUpdating)), nil
		}
		if !s.initOpArchive.isRestartPrepared() {
			return ptr.T(SimpleStatus(SyncStatusUpdating)), s.initOpArchive.prepareRestart(ctx, dry)
		}
		if !dry {
			s.setConditionOpArchivePreparedForUpdating()
		}
		return ptr.T(SimpleStatus(SyncStatusUpdating)), err
	case v1.UpdateStateWaitingForOpArchiveUpdate:
		if !s.initOpArchive.isRestartCompleted() {
			return nil, nil
		}
		if !dry {
			s.setConditionOpArchiveUpdated()
		}
		return ptr.T(SimpleStatus(SyncStatusUpdating)), err
	default:
		return nil, nil
	}
}

func (s *scheduler) needOpArchiveInit() bool {
	return s.tabletNodes != nil && len(s.tabletNodes) > 0
}

func (s *scheduler) setConditionNotNecessaryToUpdateOpArchive() {
	s.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
		Type:    consts.ConditionNotNecessaryToUpdateOpArchive,
		Status:  metav1.ConditionTrue,
		Reason:  "NotNecessaryToUpdateOpArchive",
		Message: fmt.Sprintf("Operations archive does not need to be updated"),
	})
}

func (s *scheduler) setConditionOpArchivePreparedForUpdating() {
	s.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
		Type:    consts.ConditionOpArchivePreparedForUpdating,
		Status:  metav1.ConditionTrue,
		Reason:  "OpArchivePreparedForUpdating",
		Message: fmt.Sprintf("Operations archive prepared for updating"),
	})
}

func (s *scheduler) setConditionOpArchiveUpdated() {
	s.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
		Type:    consts.ConditionOpArchiveUpdated,
		Status:  metav1.ConditionTrue,
		Reason:  "OpArchiveUpdated",
		Message: fmt.Sprintf("Operations archive updated"),
	})
}

func (s *scheduler) createInitScript() string {
	token, _ := s.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand("operation_archivarius", "", token, true)
	script := []string{
		initJobWithNativeDriverPrologue(),
	}
	script = append(script, commands...)

	return strings.Join(script, "\n")
}

func (s *scheduler) prepareInitOperationArchive() {
	script := []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf("/usr/bin/init_operation_archive --force --latest --proxy %s",
			s.cfgen.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole)),
		"/usr/bin/yt set //sys/cluster_nodes/@config '{\"%true\" = {job_agent={enable_job_reporter=%true}}}'",
	}

	s.initOpArchive.SetInitScript(strings.Join(script, "\n"))
	job := s.initOpArchive.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{s.secret.GetEnvSource()}
}
