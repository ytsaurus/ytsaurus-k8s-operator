package components

import (
	"context"
	"log"
	"path"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type baseExecNode struct {
	server    server
	cfgen     *ytconfig.NodeGenerator
	criConfig *ytconfig.CRIConfigGenerator
	spec      *ytv1.ExecNodesSpec

	sidecarConfig *ConfigMapBuilder
}

func (n *baseExecNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server, n.sidecarConfig)
}

func (n *baseExecNode) doBuildBase() error {
	statefulSet := n.server.buildStatefulSet()
	podSpec := &statefulSet.Spec.Template.Spec

	if len(podSpec.Containers) != 1 {
		log.Panicf("Number of exec node containers is expected to be 1, actual %v", len(podSpec.Containers))
	}

	if err := AddInitContainersToPodSpec(n.spec.InitContainers, podSpec); err != nil {
		return err
	}

	if err := AddSidecarsToPodSpec(n.spec.Sidecars, podSpec); err != nil {
		return err
	}

	setContainerPrivileged := func(ct *corev1.Container) {
		if ct.SecurityContext == nil {
			ct.SecurityContext = &corev1.SecurityContext{}
		}
		if ct.SecurityContext.Privileged == nil {
			ct.SecurityContext.Privileged = ptr.To(n.spec.Privileged)
		}
	}

	for i := range podSpec.InitContainers {
		setContainerPrivileged(&podSpec.InitContainers[i])
	}

	for i := range podSpec.Containers {
		setContainerPrivileged(&podSpec.Containers[i])
	}

	if n.spec.JobResources != nil && !n.criConfig.Isolated {
		// Pour job resources into node container if jobs are not isolated.
		container := &podSpec.Containers[0]

		addResourceList := func(list, newList corev1.ResourceList) {
			for name, quantity := range newList {
				if value, ok := list[name]; ok {
					value.Add(quantity)
					list[name] = value
				} else {
					list[name] = quantity.DeepCopy()
				}
			}
		}

		addResourceList(container.Resources.Requests, n.spec.JobResources.Requests)
		addResourceList(container.Resources.Limits, n.spec.JobResources.Limits)
	}

	if n.criConfig.Service != ytv1.CRIServiceNone {
		if n.criConfig.Isolated {
			// Add CRI service sidecar container for running jobs.
			n.addCRIServiceSidecar(n.criConfig, podSpec)
		} else {
			// CRI service is supposed to be started by exec node entrypoint wrapper.
			n.addCRIServiceConfig(n.criConfig, podSpec, &podSpec.Containers[0])
		}

		toolsEnv := n.criConfig.GetCRIToolsEnv()
		for i := range podSpec.Containers {
			podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, toolsEnv...)
		}
	}

	if n.sidecarConfig != nil {
		n.sidecarConfig.Build()
	}

	return nil
}

func (n *baseExecNode) addCRIServiceConfig(cri *ytconfig.CRIConfigGenerator, podSpec *corev1.PodSpec, container *corev1.Container) {
	if n.sidecarConfig == nil {
		return
	}

	switch cri.Service {
	case ytv1.CRIServiceCRIO:
		podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(
			consts.CRIOConfigVolumeName,
			n.sidecarConfig.GetConfigMapName(),
			nil,
		))
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      consts.CRIOConfigVolumeName,
			MountPath: consts.CRIOConfigMountPoint,
			ReadOnly:  true,
		})
	case ytv1.CRIServiceContainerd:
		podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(
			consts.ContainerdConfigVolumeName,
			n.sidecarConfig.GetConfigMapName(),
			nil,
		))
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      consts.ContainerdConfigVolumeName,
			MountPath: consts.ContainerdConfigMountPoint,
			ReadOnly:  true,
		})
	}

	if cri.MonitoringPort != 0 {
		container.Ports = append(container.Ports, corev1.ContainerPort{
			Name:          consts.CRIServiceMonitoringPortName,
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: cri.MonitoringPort,
		})
	}
}

func (n *baseExecNode) addCRIServiceSidecar(cri *ytconfig.CRIConfigGenerator, podSpec *corev1.PodSpec) {
	envSpec := n.spec.JobEnvironment

	wrapper := envSpec.CRI.EntrypointWrapper
	if len(wrapper) == 0 {
		wrapper = []string{"tini", "--"}
	}

	command := make([]string, 0, 1+len(wrapper))
	command = append(command, wrapper...)
	command = append(command, string(cri.Service))

	container := corev1.Container{
		Name:         consts.JobsContainerName,
		Image:        podSpec.Containers[0].Image,
		Command:      command,
		Env:          getDefaultEnv(),
		VolumeMounts: createVolumeMounts(n.spec.VolumeMounts),
		SecurityContext: &corev1.SecurityContext{
			Privileged: ptr.To(true),
		},
	}

	switch cri.Service {
	case ytv1.CRIServiceContainerd:
		configPath := path.Join(consts.ContainerdConfigMountPoint, consts.ContainerdConfigFileName)
		container.Args = []string{"--config", configPath}
	case ytv1.CRIServiceCRIO:
		configPath := path.Join(consts.CRIOConfigMountPoint, consts.CRIOConfigFileName)
		container.Args = []string{"--config", configPath, "--config-dir", ""}
	}

	n.addCRIServiceConfig(cri, podSpec, &container)

	n.server.addCARootBundle(&container)

	// Replace mount propagation "Bidirectional" -> "HostToContainer".
	// Tmpfs are propagated: exec-node -> host -> containerd.
	for i := range container.VolumeMounts {
		mount := &container.VolumeMounts[i]
		newProp := corev1.MountPropagationHostToContainer
		if prop := mount.MountPropagation; prop != nil && *prop == corev1.MountPropagationBidirectional {
			mount.MountPropagation = &newProp
		}
	}

	if n.spec.JobResources != nil {
		container.Resources = *n.spec.JobResources.DeepCopy()
	} else {
		// Without dedicated job resources enforce same limits as for node, with zero requests.
		container.Resources = corev1.ResourceRequirements{
			Limits:   n.spec.Resources.Limits.DeepCopy(),
			Requests: make(corev1.ResourceList, len(n.spec.Resources.Limits)),
		}
		for name := range container.Resources.Limits {
			container.Resources.Requests[name] = resource.Quantity{}
		}
	}

	podSpec.Containers = append(podSpec.Containers, container)
}

func (n *baseExecNode) doSyncBase(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	if !dry {
		err = n.doBuildBase()
		if err != nil {
			return ComponentStatusBlockedBy("cannot build exec node spec"), err
		}
		err = resources.Sync(ctx, n.server, n.sidecarConfig)
	}
	return ComponentStatusWaitingFor("components"), err
}

func (n *baseExecNode) sidecarConfigNeedsReload() bool {
	if n.sidecarConfig == nil {
		return false
	}

	needsReload, err := n.sidecarConfig.NeedReload()
	if err != nil {
		return false
	}
	return needsReload
}

func NewJobsSidecarConfig(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	criConfig *ytconfig.CRIConfigGenerator,
	configOverrides *corev1.LocalObjectReference,
) *ConfigMapBuilder {
	config := NewConfigMapBuilder(
		labeller,
		apiProxy,
		labeller.GetSidecarConfigMapName(consts.JobsContainerName),
		configOverrides,
	)

	switch criConfig.Service {
	case ytv1.CRIServiceNone:
		config = nil
	case ytv1.CRIServiceContainerd:
		config.AddGenerator(
			consts.ContainerdConfigFileName,
			ConfigFormatToml,
			criConfig.GetContainerdConfig,
		)
	case ytv1.CRIServiceCRIO:
		config.AddGenerator(
			consts.CRIOConfigFileName,
			ConfigFormatToml,
			criConfig.GetCRIOConfig,
		)
		config.AddGenerator(
			consts.CRIOSignaturePolicyFileName,
			ConfigFormatJson,
			criConfig.GetCRIOSignaturePolicy,
		)
	}

	return config
}
