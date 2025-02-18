package components

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type TabletBalancer struct {
	localServerComponent
	cfgen *ytconfig.Generator

	ytsaurusClient internalYtsaurusClient
	master         Component
	initCondition  string
	secret         *resources.StringSecret
}

func NewTabletBalancer(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	yc internalYtsaurusClient,
	master Component,
) *TabletBalancer {
	l := cfgen.GetComponentLabeller(consts.TabletBalancerType, "")

	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.TabletBalancer.InstanceSpec,
		"/usr/bin/ytserver-tablet-balancer",
		"ytserver-tablet-balancer.yson",
		func() ([]byte, error) {
			return cfgen.GetTabletBalancerConfig(resource.Spec.TabletBalancer)
		},
		consts.TabletBalancerMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.TabletBalancerRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &TabletBalancer{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
		initCondition:        "tabletBalancerInitCompleted",
		ytsaurusClient:       yc,
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus.APIProxy()),
	}
}

func (t *TabletBalancer) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		t.server,
		t.secret,
	)
}

func (t *TabletBalancer) Status(ctx context.Context) (ComponentStatus, error) {
	return t.doSync(ctx, true)
}

func (t *TabletBalancer) Sync(ctx context.Context) error {
	_, err := t.doSync(ctx, false)
	return err
}

func (t *TabletBalancer) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(t.ytsaurus.GetClusterState()) && t.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if t.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, t.ytsaurus, t, &t.localComponent, t.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := t.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, t.master.GetFullName()), err
	}

	if t.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			secretSpec := t.secret.Build()
			secretSpec.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = t.secret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, t.secret.Name()), err
	}

	if t.NeedSync() {
		if !dry {
			err = t.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !t.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	if t.ytsaurus.IsStatusConditionTrue(t.initCondition) {
		return SimpleStatus(SyncStatusReady), err
	}

	var ytClient yt.Client
	if t.ytsaurus.GetClusterState() != ytv1.ClusterStateUpdating {
		ytClientStatus, err := t.ytsaurusClient.Status(ctx)
		if err != nil {
			return ytClientStatus, err
		}
		if ytClientStatus.SyncStatus != SyncStatusReady {
			return WaitingStatus(SyncStatusBlocked, t.ytsaurusClient.GetFullName()), err
		}

		if !dry {
			ytClient = t.ytsaurusClient.GetYtClient()

			err = t.createUser(ctx, ytClient)
			if err != nil {
				return WaitingStatus(SyncStatusPending, "create tablet balancer user"), err
			}
		}
	}

	if !dry {
		err = t.init(ctx, ytClient)
		if err != nil {
			return WaitingStatus(SyncStatusPending, fmt.Sprintf("%s initialization", t.GetFullName())), err
		}

		t.ytsaurus.SetStatusCondition(metav1.Condition{
			Type:    t.initCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitTabletBalancerCompleted",
			Message: "Init tablet balancer successfully completed",
		})
	}

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", t.initCondition)), err
}

func (t *TabletBalancer) createUser(ctx context.Context, ytClient yt.Client) (err error) {
	logger := log.FromContext(ctx)

	token, _ := t.secret.GetValue(consts.TokenSecretKey)
	err = CreateUser(ctx, ytClient, "tablet_balancer", token, true)
	if err != nil {
		logger.Error(err, "Creating user 'tablet_balancer' failed")
		return
	}

	return
}

func (t *TabletBalancer) init(ctx context.Context, ytClient yt.Client) (err error) {
	logger := log.FromContext(ctx)

	_, err = ytClient.CreateNode(
		ctx,
		ypath.Path("//sys/tablet_balancer/config"),
		yt.NodeDocument,
		&yt.CreateNodeOptions{
			Attributes: map[string]interface{}{
				"value": map[string]interface{}{
					"enable": true,
				},
			},
			Recursive:      true,
			IgnoreExisting: true,
		},
	)
	if err != nil {
		logger.Error(err, "Creating document '//sys/tablet_balancer/config' failed")
		return
	}

	return
}
