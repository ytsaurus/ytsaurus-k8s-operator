package components

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type Scheduler struct {
	localServerComponent
	cfgen            *ytconfig.Generator
	master           Component
	execNodes        []Component
	tabletNodes      []Component
	initUserJob      *InitJob
	initOpArchiveJob *InitJob
	secret           *resources.StringSecret
}

func NewScheduler(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	execNodes, tabletNodes []Component,
) *Scheduler {
	l := cfgen.GetComponentLabeller(consts.SchedulerType, "")

	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.Schedulers.InstanceSpec,
		"/usr/bin/ytserver-scheduler",
		"ytserver-scheduler.yson",
		func() ([]byte, error) {
			return cfgen.GetSchedulerConfig(resource.Spec.Schedulers)
		},
		consts.SchedulerMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.SchedulerRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &Scheduler{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
		execNodes:            execNodes,
		tabletNodes:          tabletNodes,
		initUserJob: NewInitJob(
			l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			getImageWithDefault(resource.Spec.Schedulers.InstanceSpec.Image, resource.Spec.CoreImage),
			cfgen.GetNativeClientConfig,
			getTolerationsWithDefault(resource.Spec.Schedulers.Tolerations, resource.Spec.Tolerations),
			getNodeSelectorWithDefault(resource.Spec.Schedulers.NodeSelector, resource.Spec.NodeSelector),
			getDNSConfigWithDefault(resource.Spec.Schedulers.DNSConfig, resource.Spec.DNSConfig),
			&resource.Spec.CommonSpec,
		),
		initOpArchiveJob: NewInitJob(
			l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"op-archive",
			consts.ClientConfigFileName,
			getImageWithDefault(resource.Spec.Schedulers.InstanceSpec.Image, resource.Spec.CoreImage),
			cfgen.GetNativeClientConfig,
			getTolerationsWithDefault(resource.Spec.Schedulers.Tolerations, resource.Spec.Tolerations),
			getNodeSelectorWithDefault(resource.Spec.Schedulers.NodeSelector, resource.Spec.NodeSelector),
			getDNSConfigWithDefault(resource.Spec.Schedulers.DNSConfig, resource.Spec.DNSConfig),
			&resource.Spec.CommonSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus.APIProxy()),
	}
}

func (s *Scheduler) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		s.server,
		s.initOpArchiveJob,
		s.initUserJob,
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
		return ComponentStatusBlockedBy(s.master), nil
	}

	for _, end := range s.execNodes {
		endStatus, err := end.Status(ctx)
		if err != nil {
			return endStatus, err
		}
		if !IsRunningStatus(endStatus.SyncStatus) {
			// It makes no sense to start scheduler without exec nodes.
			// FIXME(khlebnikov): Nope, now we have remote exec nodes.
			return ComponentStatusBlockedBy(end), nil
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

	return s.initOpArchive(ctx, dry)
}

func (s *Scheduler) initOpArchive(ctx context.Context, dry bool) (ComponentStatus, error) {
	if !dry {
		s.initUserJob.SetInitScript(s.createInitUserScript())
	}

	status, err := s.initUserJob.Sync(ctx, dry)
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
			return ComponentStatusBlockedBy(tnd), nil
		}
	}

	if !dry {
		s.prepareInitOperationsArchive()
	}
	return s.initOpArchiveJob.Sync(ctx, dry)
}

func (s *Scheduler) updateOpArchive(ctx context.Context, dry bool) (*ComponentStatus, error) {
	var err error
	switch s.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare:
		if !s.needOpArchiveInit() {
			if !dry {
				s.setConditionOpArchivePreparedForUpdating(ctx)
			}
			return ptr.To(SimpleStatus(SyncStatusUpdating)), nil
		}
		if !s.initOpArchiveJob.isRestartPrepared() {
			return ptr.To(SimpleStatus(SyncStatusUpdating)), s.initOpArchiveJob.prepareRestart(ctx, dry)
		}
		if !dry {
			s.setConditionOpArchivePreparedForUpdating(ctx)
		}
		return ptr.To(SimpleStatus(SyncStatusUpdating)), err
	case ytv1.UpdateStateWaitingForOpArchiveUpdate:
		if !s.needOpArchiveInit() {
			if !dry {
				s.setConditionOpArchiveUpdated(ctx)
			}
			return ptr.To(SimpleStatus(SyncStatusUpdating)), nil
		}
		if !s.initOpArchiveJob.isRestartCompleted() {
			return nil, nil
		}
		if !dry {
			s.setConditionOpArchiveUpdated(ctx)
		}
		return ptr.To(SimpleStatus(SyncStatusUpdating)), err
	default:
		return nil, nil
	}
}

func (s *Scheduler) needOpArchiveInit() bool {
	return len(s.tabletNodes) > 0
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

// omgronny: The file was renamed in ytsaurus image. Refer to
// https://github.com/ytsaurus/ytsaurus/commit/e5348fef221110ce27bba13df5f9790649084b01
const setInitOpArchivePath = `
export INIT_OP_ARCHIVE=/usr/bin/init_operations_archive
if [ ! -f $INIT_OP_ARCHIVE ]; then
export INIT_OP_ARCHIVE=/usr/bin/init_operation_archive
fi
`

func (s *Scheduler) prepareInitOperationsArchive() {
	script := []string{
		initJobWithNativeDriverPrologue(),
		setInitOpArchivePath,
		fmt.Sprintf("$INIT_OP_ARCHIVE --force --latest --proxy %s",
			s.cfgen.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole)),
		SetWithIgnoreExisting("//sys/cluster_nodes/@config", "'{\"%true\" = {job_agent={enable_job_reporter=%true}}}'"),
	}

	s.initOpArchiveJob.SetInitScript(strings.Join(script, "\n"))
	job := s.initOpArchiveJob.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{s.secret.GetEnvSource()}
}
