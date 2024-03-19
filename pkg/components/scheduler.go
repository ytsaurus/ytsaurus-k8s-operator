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
		ComponentName:  "Scheduler",
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

func (s *Scheduler) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		s.server,
		s.initOpArchive,
		s.initUser,
		s.secret,
	)
}

func (s *Scheduler) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(s.ytsaurus.GetClusterState()) && s.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if s.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(s.ytsaurus, s) {
			if s.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				return WaitingStatus(SyncStatusUpdating, "pods removal")
			}

			if status := s.updateOpArchiveStatus(ctx); status != nil {
				return *status
			}

			if s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation &&
				s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForOpArchiveUpdate {
				return NewComponentStatus(SyncStatusReady, "Nothing to do now")
			}
		} else {
			return NewComponentStatus(SyncStatusReady, "Not updating component")
		}
	}

	if !IsRunningStatus(s.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, s.master.GetName())
	}

	if s.execNodes == nil || len(s.execNodes) > 0 {
		for _, end := range s.execNodes {
			if !IsRunningStatus(end.Status(ctx).SyncStatus) {
				// It makes no sense to start scheduler without exec nodes.
				return WaitingStatus(SyncStatusBlocked, end.GetName())
			}
		}
	}

	if s.secret.NeedSync(consts.TokenSecretKey, "") {
		return WaitingStatus(SyncStatusPending, s.secret.Name())
	}

	if s.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !s.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	if !s.needOpArchiveInit() {
		// Don't initialize operations archive.
		return SimpleStatus(SyncStatusReady)
	}

	return s.initOpAchieveStatus(ctx)
}

func (s *Scheduler) Sync(ctx context.Context) error {
	if ytv1.IsReadyToUpdateClusterState(s.ytsaurus.GetClusterState()) && s.server.needUpdate() {
		return nil
	}

	if s.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(s.ytsaurus, s) {
			if s.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				return removePods(ctx, s.server, &s.localComponent)
			}

			if err := s.updateOpArchiveSync(ctx); err != nil {
				return err
			}

			if s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation &&
				s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForOpArchiveUpdate {
				return nil
			}
		} else {
			return nil
		}
	}

	if !IsRunningStatus(s.master.Status(ctx).SyncStatus) {
		return nil
	}

	if s.execNodes == nil || len(s.execNodes) > 0 {
		for _, end := range s.execNodes {
			if !IsRunningStatus(end.Status(ctx).SyncStatus) {
				// It makes no sense to start scheduler without exec nodes.
				return nil
			}
		}
	}

	if s.secret.NeedSync(consts.TokenSecretKey, "") {
		secretSpec := s.secret.Build()
		secretSpec.StringData = map[string]string{
			consts.TokenSecretKey: ytconfig.RandString(30),
		}
		return s.secret.Sync(ctx)
	}

	if s.NeedSync() {
		return s.server.Sync(ctx)
	}

	if !s.server.arePodsReady(ctx) {
		return nil
	}

	if !s.needOpArchiveInit() {
		// Don't initialize operations archive.
		return nil
	}

	return s.initOpAchieveSync(ctx)
}

func (s *Scheduler) initOpAchieveStatus(ctx context.Context) ComponentStatus {
	status, err := s.initUser.Sync(ctx, true)
	if status.SyncStatus != SyncStatusReady {
		if err != nil {
			panic(err)
		}
		return status
	}

	for _, tnd := range s.tabletNodes {
		if !IsRunningStatus(tnd.Status(ctx).SyncStatus) {
			// Wait for tablet nodes to proceed with operations archive init.
			return WaitingStatus(SyncStatusBlocked, tnd.GetName())
		}
	}

	st, err := s.initOpArchive.Sync(ctx, true)
	if err != nil {
		panic(err)
	}
	return st
}

func (s *Scheduler) initOpAchieveSync(ctx context.Context) error {
	s.initUser.SetInitScript(s.createInitUserScript())

	status, err := s.initUser.Sync(ctx, false)
	if status.SyncStatus != SyncStatusReady {
		return err
	}

	for _, tnd := range s.tabletNodes {
		if !IsRunningStatus(tnd.Status(ctx).SyncStatus) {
			// Wait for tablet nodes to proceed with operations archive init.
			return nil
		}
	}

	s.prepareInitOperationArchive()
	_, err = s.initOpArchive.Sync(ctx, false)
	return err
}

func (s *Scheduler) updateOpArchiveStatus(ctx context.Context) *ComponentStatus {
	switch s.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare:
		if !s.needOpArchiveInit() {
			s.setConditionNotNecessaryToUpdateOpArchive(ctx)
			return ptr.T(SimpleStatus(SyncStatusUpdating))
		}
		if !s.initOpArchive.isRestartPrepared() {
			err := s.initOpArchive.prepareRestart(ctx, true)
			if err != nil {
				panic(err)
			}
			return ptr.T(SimpleStatus(SyncStatusUpdating))
		}
		return ptr.T(SimpleStatus(SyncStatusUpdating))
	case ytv1.UpdateStateWaitingForOpArchiveUpdate:
		if !s.initOpArchive.isRestartCompleted() {
			return nil
		}
		return ptr.T(SimpleStatus(SyncStatusUpdating))
	default:
		return nil
	}
}

func (s *Scheduler) updateOpArchiveSync(ctx context.Context) error {
	switch s.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare:
		if !s.needOpArchiveInit() {
			s.setConditionNotNecessaryToUpdateOpArchive(ctx)
			return nil
		}
		if !s.initOpArchive.isRestartPrepared() {
			return s.initOpArchive.prepareRestart(ctx, false)
		}
		s.setConditionOpArchivePreparedForUpdating(ctx)
		return nil
	case ytv1.UpdateStateWaitingForOpArchiveUpdate:
		if !s.initOpArchive.isRestartCompleted() {
			return nil
		}
		s.setConditionOpArchiveUpdated(ctx)
		return nil
	default:
		return nil
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
		Message: fmt.Sprintf("Operations archive does not need to be updated"),
	})
}

func (s *Scheduler) setConditionOpArchivePreparedForUpdating(ctx context.Context) {
	s.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionOpArchivePreparedForUpdating,
		Status:  metav1.ConditionTrue,
		Reason:  "OpArchivePreparedForUpdating",
		Message: fmt.Sprintf("Operations archive prepared for updating"),
	})
}

func (s *Scheduler) setConditionOpArchiveUpdated(ctx context.Context) {
	s.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionOpArchiveUpdated,
		Status:  metav1.ConditionTrue,
		Reason:  "OpArchiveUpdated",
		Message: fmt.Sprintf("Operations archive updated"),
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
