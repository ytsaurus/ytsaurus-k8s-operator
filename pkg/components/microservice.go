package components

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
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
}

type microserviceImpl struct {
	image         string
	labeller      *labeller.Labeller
	instanceCount int32
	ytsaurus      *apiproxy.Ytsaurus

	deployment *resources.Deployment
	service    *resources.HTTPService
	configs    *ConfigMapBuilder

	builtDeployment *appsv1.Deployment
	builtService    *corev1.Service
}

func newMicroservice(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	image string,
	instanceCount int32,
	generators map[string]ConfigGenerator,
	deploymentName, serviceName string,
	tolerations []corev1.Toleration,
	nodeSelector map[string]string,
) microservice {
	configs := NewConfigMapBuilder(
		labeller,
		ytsaurus.APIProxy(),
		labeller.GetMainConfigMapName(),
		ytsaurus.GetResource().Spec.ConfigOverrides,
	)

	for fileName, generator := range generators {
		configs.AddGenerator(fileName, generator.Format, generator.Generator)
	}

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
			tolerations,
			nodeSelector,
		),
		configs: configs,
	}
}

func (m *microserviceImpl) Sync(ctx context.Context) error {
	cm, err := m.configs.Build()
	if err != nil {
		return err
	}
	_ = m.buildDeployment()
	_ = m.buildService()

	if err := m.setConfigHash(cm); err != nil {
		return err
	}

	if err := m.setInstanceHash(); err != nil {
		return err
	}

	return resources.Sync(ctx,
		m.deployment,
		m.configs,
		m.service,
	)
}

func (m *microserviceImpl) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		m.configs,
		m.deployment,
		m.service,
	)
}

func (m *microserviceImpl) needSync() bool {
	needReload, err := m.configs.NeedReload()
	if err != nil {
		needReload = false
	}
	return m.configs.NeedInit() ||
		(m.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating && needReload) ||
		!resources.Exists(m.service) ||
		m.deployment.GetReplicas() != m.instanceCount
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

func (m *microserviceImpl) setConfigHash(cm *corev1.ConfigMap) error {
	configHash := cm.Annotations[consts.ConfigHashAnnotationName]
	if configHash == "" {
		return fmt.Errorf("config hash is not found for %v", cm.Name)
	}
	// Propagate config hash from configmap to pod template to trigger restart when needed.
	metav1.SetMetaDataAnnotation(&m.builtDeployment.Spec.Template.ObjectMeta, consts.ConfigHashAnnotationName, configHash)
	return nil
}

func (m *microserviceImpl) setInstanceHash() error {
	instanceHash, err := resources.Hash(&m.builtDeployment.Spec.Template)
	if err != nil {
		return err
	}
	metav1.SetMetaDataAnnotation(&m.builtDeployment.ObjectMeta, consts.InstanceHashAnnotationName, instanceHash)
	return nil
}

func (m *microserviceImpl) buildService() *corev1.Service {
	if m.builtService == nil {
		m.builtService = m.service.Build()
	}
	return m.builtService
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

func (m *microserviceImpl) arePodsUpdatedToNewRevision(ctx context.Context) bool {
	// For now, return true
	return true
}

func (m *microserviceImpl) needUpdate() bool {
	if !m.exists() {
		return false
	}

	if !m.podsImageCorrespondsToSpec() {
		return true
	}

	needReload, err := m.configs.NeedReload()
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
	return m.deployment.OldObject().Spec.Template.Spec.Containers[0].Image == m.image
}

func (m *microserviceImpl) removePods(ctx context.Context) error {
	m.builtDeployment = m.deployment.Build()
	m.builtDeployment.Spec = m.deployment.OldObject().Spec
	m.builtDeployment.Spec.Replicas = ptr.To(int32(0))
	return m.Sync(ctx)
}

func (m *microserviceImpl) getImage() string {
	return m.image
}
