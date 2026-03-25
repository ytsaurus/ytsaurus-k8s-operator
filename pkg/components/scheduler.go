package components

import (
	"context"
	"fmt"
	"strings"

	"go.ytsaurus.tech/yt/go/ypath"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	schedulerAlertsPath = "//sys/scheduler/@alerts"
	orchidPath          = "//sys/scheduler/orchid"
)

type Scheduler struct {
	serverComponent

	cfgen            *ytconfig.Generator
	master           Component
	tabletNodes      []Component
	ytsaurusClient   internalYtsaurusClient
	initUserJob      *InitJob
	initOpArchiveJob *InitJob
	secret           *resources.StringSecret
}

func NewScheduler(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	yc internalYtsaurusClient,
	tabletNodes []Component,
) *Scheduler {
	l := cfgen.GetComponentLabeller(consts.SchedulerType, "")

	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.Schedulers.InstanceSpec,
		"/usr/bin/ytserver-scheduler",
		[]ConfigGenerator{{
			"ytserver-scheduler.yson",
			ConfigFormatYson,
			func() ([]byte, error) { return cfgen.GetSchedulerConfig(resource.Spec.Schedulers) },
		}},
		consts.SchedulerMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.SchedulerRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &Scheduler{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
		master:          master,
		tabletNodes:     tabletNodes,
		ytsaurusClient:  yc,
		initUserJob: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"user",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&resource.Spec.Schedulers.InstanceSpec,
		),
		initOpArchiveJob: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"op-archive",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&resource.Spec.Schedulers.InstanceSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus),
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

func (s *Scheduler) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if s.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := dispatchComponentUpdate(ctx, s.ytsaurus, s, &s.component, s.server, dry); status != nil {
			return *status, err
		}

		if status, err := s.updateOpArchive(ctx, dry); status != nil {
			return *status, err
		}

		if s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation &&
			s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForOpArchiveUpdate {
			return ComponentStatusReady(), nil
		}
	}

	if masterStatus := s.master.GetStatus(); !masterStatus.IsRunning() {
		return masterStatus.Blocker(), nil
	}

	if s.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			secretSpec := s.secret.Build()
			secretSpec.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = s.secret.Sync(ctx)
		}
		return ComponentStatusWaitingFor(s.secret.Name()), err
	}

	if s.NeedSync() {
		if !dry {
			err = s.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if status, err := s.ArePodsReady(ctx); !status.IsReady() || err != nil {
		return status, err
	}

	// FIXME: Refactor this mess. During update flow sync must do only actions for current update phase.
	if s.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating && s.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsCreation {
		return ComponentStatusReady(), nil
	}

	if !s.needOpArchiveInit() {
		// Don't initialize operations archive.
		return ComponentStatusReady(), err
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
		if tndStatus := tnd.GetStatus(); !tndStatus.IsRunning() {
			// Wait for tablet nodes to proceed with operations archive init.
			return tndStatus.Blocker(), nil
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

func (s *Scheduler) UpdatePreCheck(ctx context.Context) ComponentStatus {
	logger := log.FromContext(ctx)

	if s.ytsaurusClient == nil {
		return ComponentStatusBlocked("YtsaurusClient component is not available")
	}

	ytClient := s.ytsaurusClient.GetYtClient()
	if ytClient == nil {
		return ComponentStatusBlocked("YT client is not available")
	}

	// Check for scheduler alerts
	var schedulerAlerts []any
	if err := ytClient.GetNode(ctx, ypath.Path(schedulerAlertsPath), &schedulerAlerts, nil); err != nil {
		return ComponentStatusBlocked("Could not check scheduler alerts: %v", err)
	}

	alertCount := len(schedulerAlerts)
	if alertCount > 0 {
		return ComponentStatusBlocked("Scheduler has %d alerts: %v", alertCount, schedulerAlerts)
	}

	// Try to get orchid data
	var orchidData map[string]interface{}
	if err := ytClient.GetNode(ctx, ypath.Path(orchidPath), &orchidData, nil); err != nil {
		return ComponentStatusBlocked("Failed to get scheduler orchid data: %v", err)
	}
	logger.Info("Scheduler orchid data retrieved successfully")

	return ComponentStatusReady()
}
