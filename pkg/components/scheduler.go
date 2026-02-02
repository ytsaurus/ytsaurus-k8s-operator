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
	localServerComponent
	cfgen            *ytconfig.Generator
	master           Component
	execNodes        []Component
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
		ytsaurusClient:       yc,
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

	if s.ytsaurus.IsReadyToUpdate() && s.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if s.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(s.ytsaurus, s) {
			switch getComponentUpdateStrategy(s.ytsaurus, consts.SchedulerType, s.GetShortName()) {
			case ytv1.ComponentUpdateModeTypeOnDelete:
				if status, err := handleOnDeleteUpdatingClusterState(ctx, s.ytsaurus, s, &s.localComponent, s.server, dry); status != nil {
					return *status, err
				}
			default:
				if status, err := handleBulkUpdatingClusterState(ctx, s.ytsaurus, s, &s.localComponent, s.server, dry); status != nil {
					return *status, err
				}
			}

			if status, err := s.updateOpArchive(ctx, dry); status != nil {
				return *status, err
			}

			if s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation &&
				s.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForOpArchiveUpdate {
				return ComponentStatusReady(), err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	masterStatus, err := s.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !masterStatus.IsRunning() {
		return ComponentStatusBlockedBy(s.master.GetFullName()), err
	}

	for _, end := range s.execNodes {
		endStatus, err := end.Status(ctx)
		if err != nil {
			return endStatus, err
		}
		if !endStatus.IsRunning() {
			// It makes no sense to start scheduler without exec nodes.
			return ComponentStatusBlockedBy(end.GetFullName()), err
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
		return ComponentStatusWaitingFor(s.secret.Name()), err
	}

	if s.NeedSync() {
		if !dry {
			err = s.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !s.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
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
		tndStatus, err := tnd.Status(ctx)
		if err != nil {
			return tndStatus, err
		}
		if !tndStatus.IsRunning() {
			// Wait for tablet nodes to proceed with operations archive init.
			return ComponentStatusBlockedBy(tnd.GetFullName()), err
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
		return ComponentStatusBlocked(fmt.Sprintf("Could not check scheduler alerts: %v", err))
	}

	alertCount := len(schedulerAlerts)
	if alertCount > 0 {
		return ComponentStatusBlocked(fmt.Sprintf("Scheduler has %d alerts: %v", alertCount, schedulerAlerts))
	}

	// Try to get orchid data
	var orchidData map[string]interface{}
	if err := ytClient.GetNode(ctx, ypath.Path(orchidPath), &orchidData, nil); err != nil {
		return ComponentStatusBlocked(fmt.Sprintf("Failed to get scheduler orchid data: %v", err))
	}
	logger.Info("Scheduler orchid data retrieved successfully")

	return ComponentStatusReady()
}
