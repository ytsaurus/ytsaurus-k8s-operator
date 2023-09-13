package components

import (
	"context"
	ptr "k8s.io/utils/pointer"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// microservice manages common resources of YTsaurus service component
// that typically is not directly presented in the cypress
// and doesn't use native connection to communicate with the cluster.
// Examples are YT UI and CHYT controller.
type microservice interface {
	resources.Fetchable
	resources.Syncable
	podsManager
	needSync() bool
	needUpdate() bool
	getImage() string
	buildDeployment() *appsv1.Deployment
	buildService() *corev1.Service
	buildConfig() *corev1.ConfigMap
}

type microserviceImpl struct {
	image         string
	labeller      *labeller.Labeller
	instanceCount int32

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
	configGenerator ytconfig.GeneratorFunc,
	configFileName, deploymentName, serviceName string) microservice {
	return &microserviceImpl{
		labeller:      labeller,
		image:         image,
		instanceCount: instanceCount,
		service: resources.NewHTTPService(
			serviceName,
			labeller,
			ytsaurus.APIProxy()),
		deployment: resources.NewDeployment(
			deploymentName,
			labeller,
			ytsaurus),
		configHelper: NewConfigHelper(
			labeller,
			ytsaurus.APIProxy(),
			labeller.GetMainConfigMapName(),
			configFileName,
			ytsaurus.GetResource().Spec.ConfigOverrides,
			configGenerator),
	}
}

func (m *microserviceImpl) Sync(ctx context.Context) (err error) {
	_ = m.buildConfig()
	_ = m.buildDeployment()
	_ = m.buildService()

	return resources.Sync(ctx, []resources.Syncable{
		m.deployment,
		m.configHelper,
		m.service,
	})
}

func (m *microserviceImpl) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		m.configHelper,
		m.deployment,
		m.service,
	})
}

func (m *microserviceImpl) needSync() bool {
	return m.configHelper.NeedSync() ||
		!resources.Exists(m.service) ||
		m.deployment.NeedSync(m.instanceCount)
}

func (m *microserviceImpl) buildDeployment() *appsv1.Deployment {
	if m.builtDeployment != nil {
		return m.builtDeployment
	}

	return m.rebuildDeployment()
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

func (m *microserviceImpl) arePodsRemoved() bool {
	if m.configHelper.NeedSync() || !resources.Exists(m.deployment) || !resources.Exists(m.service) {
		return false
	}

	return !m.deployment.NeedSync(0)
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
	m.builtDeployment.Spec.Replicas = ptr.Int32(0)
	return m.Sync(ctx)
}

func (m *microserviceImpl) getImage() string {
	return m.image
}
