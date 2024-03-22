package components

import (
	"context"
	"fmt"
	"strings"

	"go.ytsaurus.tech/library/go/ptr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type Scheduler struct {
	localServerComponent
	cfgen         *ytconfig.Generator
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
	execNodes, tabletNodes []Component) *Scheduler {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelScheduler,
		ComponentName:  string(consts.SchedulerType),
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	if resource.Spec.Schedulers.InstanceSpec.MonitoringPort == nil {
		resource.Spec.Schedulers.InstanceSpec.MonitoringPort = ptr.Int32(consts.SchedulerMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.Schedulers.InstanceSpec,
		"/usr/bin/ytserver-scheduler",
		"ytserver-scheduler.yson",
		cfgen.GetSchedulerStatefulSetName(),
		cfgen.GetSchedulerServiceName(),
		func() ([]byte, error) {
			return cfgen.GetSchedulerConfig(resource.Spec.Schedulers)
		},
	)

	return &Scheduler{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
		execNodes:            execNodes,
		tabletNodes:          tabletNodes,
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

func (s *Scheduler) IsUpdatable() bool {
	return true
}

func (s *Scheduler) GetType() consts.ComponentType { return consts.SchedulerType }

func (s *Scheduler) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		s.server,
		s.initOpArchive,
		s.initUser,
		s.secret,
	)
}

func (s *Scheduler) Status(ctx context.Context) (ComponentStatus, error) {
	return s.doSync(ctx, true)
}

func (s *Scheduler) Sync(ctx context.Context) error {
	_, err := s.doSync(ctx, false)
	return err
}

func (s *Scheduler) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(s.ytsaurus.GetClusterState()) && s.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if s.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(s.ytsaurus, s) {
			if s.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				if !dry {
					err = removePods(ctx, s.server, &s.localComponent)
				}
				return WaitingStatus(SyncStatusUpdating, "pods removal"), err
			}

			if status, err := s.updateOpArchive(ctx, dry); status != nil {
				return *status, err
			}

			if s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation &&
				s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForOpArchiveUpdate {
				return NewComponentStatus(SyncStatusReady, "Nothing to do now"), err
			}
		} else {
			return NewComponentStatus(SyncStatusReady, "Not updating component"), err
		}
	}

	masterStatus, err := s.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, s.master.GetName()), err
	}

	if s.execNodes == nil || len(s.execNodes) > 0 {
		for _, end := range s.execNodes {
			endStatus, err := end.Status(ctx)
			if err != nil {
				return endStatus, err
			}
			if !IsRunningStatus(endStatus.SyncStatus) {
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

	if s.NeedSync() {
		if !dry {
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

	return s.initOpAchieve(ctx, dry)
}

func (s *Scheduler) initOpAchieve(ctx context.Context, dry bool) (ComponentStatus, error) {
	if !dry {
		s.initUser.SetInitScript(s.createInitUserScript())
	}

	status, err := s.initUser.Sync(ctx, dry)
	if status.SyncStatus != SyncStatusReady {
		return status, err
	}

	for _, tnd := range s.tabletNodes {
		tndStatus, err := tnd.Status(ctx)
		if err != nil {
			return tndStatus, err
		}
		if !IsRunningStatus(tndStatus.SyncStatus) {
			// Wait for tablet nodes to proceed with operations archive init.
			return WaitingStatus(SyncStatusBlocked, tnd.GetName()), err
		}
	}

	if !dry {
		s.prepareInitOperationArchive()
	}
	return s.initOpArchive.Sync(ctx, dry)
}

func (s *Scheduler) updateOpArchive(ctx context.Context, dry bool) (*ComponentStatus, error) {
	var err error
	switch s.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare:
		if !s.needOpArchiveInit() {
			s.setConditionNotNecessaryToUpdateOpArchive(ctx)
			return ptr.T(SimpleStatus(SyncStatusUpdating)), nil
		}
		if !s.initOpArchive.isRestartPrepared() {
			return ptr.T(SimpleStatus(SyncStatusUpdating)), s.initOpArchive.prepareRestart(ctx, dry)
		}
		if !dry {
			s.setConditionOpArchivePreparedForUpdating(ctx)
		}
		return ptr.T(SimpleStatus(SyncStatusUpdating)), err
	case ytv1.UpdateStateWaitingForOpArchiveUpdate:
		if !s.initOpArchive.isRestartCompleted() {
			return nil, nil
		}
		if !dry {
			s.setConditionOpArchiveUpdated(ctx)
		}
		return ptr.T(SimpleStatus(SyncStatusUpdating)), err
	default:
		return nil, nil
	}
}

func (s *Scheduler) needOpArchiveInit() bool {
	return s.tabletNodes != nil && len(s.tabletNodes) > 0
}

func (s *Scheduler) setConditionNotNecessaryToUpdateOpArchive(ctx context.Context) {
	s.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionNotNecessaryToUpdateOpArchive,
		Status:  metav1.ConditionTrue,
		Reason:  "NotNecessaryToUpdateOpArchive",
		Message: "Operations archive does not need to be updated",
	})
}

func (s *Scheduler) setConditionOpArchivePreparedForUpdating(ctx context.Context) {
	s.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionOpArchivePreparedForUpdating,
		Status:  metav1.ConditionTrue,
		Reason:  "OpArchivePreparedForUpdating",
		Message: "Operations archive prepared for updating",
	})
}

func (s *Scheduler) setConditionOpArchiveUpdated(ctx context.Context) {
	s.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionOpArchiveUpdated,
		Status:  metav1.ConditionTrue,
		Reason:  "OpArchiveUpdated",
		Message: "Operations archive updated",
	})
}

func (s *Scheduler) createInitUserScript() string {
	token, _ := s.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand("operation_archivarius", "", token, true)
	script := []string{
		initJobWithNativeDriverPrologue(),
	}
	script = append(script, commands...)

	return strings.Join(script, "\n")
}

func (s *Scheduler) prepareInitOperationArchive() {
	script := []string{
		initJobWithNativeDriverPrologue(),
		fmt.Sprintf("/usr/bin/init_operation_archive --force --latest --proxy %s",
			s.cfgen.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole)),
		SetWithIgnoreExisting("//sys/cluster_nodes/@config", "'{\"%true\" = {job_agent={enable_job_reporter=%true}}}'"),
	}

	s.initOpArchive.SetInitScript(strings.Join(script, "\n"))
	job := s.initOpArchive.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{s.secret.GetEnvSource()}
}
