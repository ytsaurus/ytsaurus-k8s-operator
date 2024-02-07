package components

// TODO: remove ytsaurus dep later
// TODO: jobs must implement that interface somehow or maybe they are should be checked
//  by big component
// TODO: master extra before/after update stuff must be done somehow

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type Master2 struct {
	componentBase
	server server

	// TODO: some generic list of resources which has Status/Sync
	initJob          *InitJob
	exitReadOnlyJob  *InitJob
	adminCredentials corev1.Secret
}

func NewMaster2(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *Master2 {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelMaster,
		ComponentName:  "Master",
		MonitoringPort: consts.MasterMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.PrimaryMasters.InstanceSpec,
		"/usr/bin/ytserver-master",
		"ytserver-master.yson",
		cfgen.GetMastersStatefulSetName(),
		cfgen.GetMastersServiceName(),
		cfgen.GetMasterConfig,
	)

	initJob := NewInitJob(
		&l,
		ytsaurus.APIProxy(),
		ytsaurus,
		resource.Spec.ImagePullSecrets,
		"default",
		consts.ClientConfigFileName,
		resource.Spec.CoreImage,
		cfgen.GetNativeClientConfig,
	)

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

	return &Master2{
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		server:          srv,
		initJob:         initJob,
		exitReadOnlyJob: exitReadOnlyJob,
	}
}

func (m *Master2) Fetch(ctx context.Context) error {
	// TODO: 1) make secret fetchable object
	// 2) create `resources` slice and add secret according to Spec.AdminCredentials
	// 3) this method should be resources.Fetch(ctx, m.resources...)
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

func (m *Master2) Status(ctx context.Context) ComponentStatus {
	components := []SubComponent{
		m.server,
		m.initJob,
		m.exitReadOnlyJob,
	}

	for _, comp := range components {
		syncStatus := comp.Status(ctx).SyncStatus
		if syncStatus != SyncStatusReady {
			return ComponentStatus{
				SyncStatus: syncStatus,
				Message:    comp.GetName() + " is not ready",
			}
		}
	}

	return ComponentStatus{
		SyncStatus: SyncStatusReady,
	}
}

func (m *Master2) Sync2(ctx context.Context) error {
	components := []SubComponent{
		m.server,
		m.initJob,
		m.exitReadOnlyJob,
	}
	for _, comp := range components {
		syncStatus := comp.Status(ctx).SyncStatus
		if syncStatus != SyncStatusReady {
			return comp.Sync2(ctx)
		}
	}
	return nil
}
