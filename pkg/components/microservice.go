package components

import (
	"context"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

// microservice manages common resources of YTsaurus service component
// that typically is not directly presented in Cypress
// and doesn't use native connection to communicate with the cluster.
// Examples are YT UI and CHYT controller.
type microservice interface {
	resources.Fetchable
	resources.Syncable
	podsManager
	needSync() bool
	needUpdate() bool
	getImage() string
	getHttpService() *resources.HTTPService
	buildDeployment() *appsv1.Deployment
	buildService() *corev1.Service
	buildConfig() *corev1.ConfigMap
}

type microserviceImpl struct {
	image         string
	labeller      *labeller.Labeller
	instanceCount int32
	ytsaurus      *apiproxy.Ytsaurus

	deployment   *resources.Deployment
	service      *resources.HTTPService
	configHelper *ConfigHelper

	builtDeployment *appsv1.Deployment
	builtService    *corev1.Service
	builtConfig     *corev1.ConfigMap
}

func newMicroservice(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	image string,
	instanceCount int32,
	generators map[string]ytconfig.GeneratorDescriptor,
	deploymentName, serviceName string,
	tolerations []corev1.Toleration) microservice {
	return &microserviceImpl{
		labeller:      labeller,
		image:         image,
		instanceCount: instanceCount,
		ytsaurus:      ytsaurus,
		service: resources.NewHTTPService(
			serviceName,
			nil,
			labeller,
			ytsaurus.APIProxy()),
		deployment: resources.NewDeployment(
			deploymentName,
			labeller,
			ytsaurus,
			tolerations),
		configHelper: NewConfigHelper(
			labeller,
			ytsaurus.APIProxy(),
			labeller.GetMainConfigMapName(),
			ytsaurus.GetResource().Spec.ConfigOverrides,
			generators),
	}
}

func (m *microserviceImpl) Sync(ctx context.Context) (err error) {
	_ = m.buildConfig()
	_ = m.buildDeployment()
	_ = m.buildService()

	return resources.Sync(ctx,
		m.deployment,
		m.configHelper,
		m.service,
	)
}

func (m *microserviceImpl) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		m.configHelper,
		m.deployment,
		m.service,
	)
}

func (m *microserviceImpl) needSync() bool {
	needReload, err := m.configHelper.NeedReload()
	if err != nil {
		needReload = false
	}
	return m.configHelper.NeedInit() ||
		(m.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating && needReload) ||
		!resources.Exists(m.service) ||
		m.deployment.NeedSync(m.instanceCount)
}

func (m *microserviceImpl) buildDeployment() *appsv1.Deployment {
	if m.builtDeployment != nil {
		return m.builtDeployment
	}

	return m.rebuildDeployment()
}

func (m *microserviceImpl) getHttpService() *resources.HTTPService {
	return m.service
}

func (m *microserviceImpl) rebuildDeployment() *appsv1.Deployment {
	m.builtDeployment = m.deployment.Build()
	m.builtDeployment.Spec.Replicas = &m.instanceCount
	m.builtDeployment.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Image: m.image,
		},
	}
	return m.builtDeployment
}

func (m *microserviceImpl) buildService() *corev1.Service {
	if m.builtService == nil {
		m.builtService = m.service.Build()
	}
	return m.builtService
}

func (m *microserviceImpl) buildConfig() *corev1.ConfigMap {
	if m.builtConfig == nil {
		m.builtConfig = m.configHelper.Build()
	}
	return m.builtConfig
}

func (m *microserviceImpl) arePodsReady(ctx context.Context) bool {
	return m.deployment.ArePodsReady(ctx)
}

func (m *microserviceImpl) arePodsRemoved(ctx context.Context) bool {
	if !resources.Exists(m.deployment) {
		return true
	}
	return m.deployment.ArePodsRemoved(ctx)
}

func (m *microserviceImpl) needUpdate() bool {
	if !m.exists() {
		return false
	}

	if !m.podsImageCorrespondsToSpec() {
		return true
	}

	needReload, err := m.configHelper.NeedReload()
	if err != nil {
		return false
	}
	return needReload
}

func (m *microserviceImpl) exists() bool {
	return resources.Exists(m.deployment) &&
		resources.Exists(m.service)
}

func (m *microserviceImpl) podsImageCorrespondsToSpec() bool {
	return m.deployment.OldObject().(*appsv1.Deployment).Spec.Template.Spec.Containers[0].Image == m.image
}

func (m *microserviceImpl) removePods(ctx context.Context) error {
	m.builtDeployment = m.deployment.Build()
	m.builtDeployment.Spec = m.deployment.OldObject().(*appsv1.Deployment).Spec
	m.builtDeployment.Spec.Replicas = ptr.To(int32(0))
	return m.Sync(ctx)
}

func (m *microserviceImpl) getImage() string {
	return m.image
}
