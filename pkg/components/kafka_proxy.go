package components

import (
	"context"

	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type KafkaProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator

	master Component

	serviceType      *corev1.ServiceType
	balancingService *resources.RPCService
}

func NewKafkaProxy(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, masterReconciler Component, spec ytv1.KafkaProxiesSpec) *KafkaProxy {
	l := cfgen.GetComponentLabeller(consts.KafkaProxyType, spec.Role)

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.KafkaProxyMonitoringPort))
	}

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-kafka-proxy",
		"ytserver-kafka-proxy.yson",
		func() ([]byte, error) {
			return cfgen.GetKafkaProxyConfig(spec)
		},
	)

	var balancingService *resources.RPCService = nil
	if spec.ServiceType != nil {
		balancingService = resources.NewRPCService(
			cfgen.GetKafkaProxiesServiceName(spec.Role),
			l,
			ytsaurus.APIProxy())
		balancingService.SetPort(pointer.Int32(consts.KafkaProxyKafkaPort))
	}

	return &KafkaProxy{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               masterReconciler,
		serviceType:          spec.ServiceType,
		balancingService:     balancingService,
	}
}

func (kp *KafkaProxy) IsUpdatable() bool {
	return true
}

func (kp *KafkaProxy) Fetch(ctx context.Context) error {
	fetchable := []resources.Fetchable{
		kp.server,
	}
	if kp.balancingService != nil {
		fetchable = append(fetchable, kp.balancingService)
	}
	return resources.Fetch(ctx, fetchable...)
}

func (kp *KafkaProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(kp.ytsaurus.GetClusterState()) && kp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if kp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, kp.ytsaurus, kp, &kp.localComponent, kp.server, dry); status != nil {
			return *status, err
		}
	}

	kpStatus, err := kp.master.Status(ctx)
	if err != nil {
		return kpStatus, err
	}
	if !IsRunningStatus(kpStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, kp.master.GetName()), err
	}

	if kp.NeedSync() {
		if !dry {
			err = kp.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if kp.balancingService != nil && !resources.Exists(kp.balancingService) {
		if !dry {
			kp.balancingService.Build()
			err = kp.balancingService.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, kp.balancingService.Name()), err
	}

	if !kp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (kp *KafkaProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return kp.doSync(ctx, true)
}

func (kp *KafkaProxy) Sync(ctx context.Context) error {
	_, err := kp.doSync(ctx, false)
	return err
}
