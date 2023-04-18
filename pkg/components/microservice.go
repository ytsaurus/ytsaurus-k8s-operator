package components

import (
	"context"

	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Reconciles an Ytsaurus service that typically is not directly presented
// in the cypress and doesn't use native connection to communicate with the cluster.
// Examples are YT UI and CHYT controller.

type Microservice struct {
	labeller *labeller.Labeller

	image         string
	instanceCount int32

	service      *resources.HTTPService
	deployment   *resources.Deployment
	configHelper *ConfigHelper

	builtDeployment *appsv1.Deployment
	builtService    *corev1.Service
	builtConfig     *corev1.ConfigMap
}

func NewMicroservice(
	labeller *labeller.Labeller,
	apiProxy *apiproxy.APIProxy,
	image string,
	instanceCount int32,
	configGenerator ytconfig.GeneratorFunc,
	configFileName, deploymentName, serviceName string) *Microservice {
	return &Microservice{
		labeller:      labeller,
		image:         image,
		instanceCount: instanceCount,
		service: resources.NewHTTPService(
			serviceName,
			labeller,
			apiProxy),
		deployment: resources.NewDeployment(
			deploymentName,
			labeller,
			apiProxy),
		configHelper: NewConfigHelper(
			labeller,
			apiProxy,
			labeller.GetMainConfigMapName(),
			configFileName,
			configGenerator),
	}
}

func (m *Microservice) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		m.configHelper,
		m.deployment,
		m.service,
	})
}

func (m *Microservice) IsInSync() bool {
	if m.configHelper.NeedSync() ||
		!resources.Exists(m.service) {
		return false
	}

	return !m.deployment.NeedSync(m.instanceCount, m.image)
}

func (m *Microservice) BuildDeployment() *appsv1.Deployment {
	if m.builtDeployment == nil {
		m.builtDeployment = m.deployment.Build()
		m.builtDeployment.Spec.Replicas = &m.instanceCount
		m.builtDeployment.Spec.Template.Spec.Containers = []corev1.Container{
			corev1.Container{
				Image: m.image,
			},
		}
	}

	return m.builtDeployment
}

func (m *Microservice) BuildService() *corev1.Service {
	if m.builtService == nil {
		m.builtService = m.service.Build()
	}
	return m.builtService
}

func (m *Microservice) BuildConfig() *corev1.ConfigMap {
	if m.builtConfig == nil {
		m.builtConfig = m.configHelper.Build()
	}
	return m.builtConfig
}

func (m *Microservice) Sync(ctx context.Context) (err error) {
	_ = m.BuildConfig()
	_ = m.BuildDeployment()
	_ = m.BuildService()

	return resources.Sync(ctx, []resources.Syncable{
		m.deployment,
		m.configHelper,
		m.service,
	})
}
