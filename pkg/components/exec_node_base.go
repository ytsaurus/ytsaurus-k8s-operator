package components

import (
	"context"
	"log"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
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
			n.addCRIServiceConfig(n.criConfig, &podSpec.Containers[0])
		}

		toolsEnv := n.criConfig.GetCRIToolsEnv()
		for i := range podSpec.Containers {
			podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, toolsEnv...)
		}
	}

	if n.sidecarConfig != nil {
		podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(consts.ContainerdConfigVolumeName,
			n.sidecarConfig.labeller.GetSidecarConfigMapName(consts.JobsContainerName), nil))

		n.sidecarConfig.Build()
	}

	return nil
}

func (n *baseExecNode) addCRIServiceConfig(cri *ytconfig.CRIConfigGenerator, container *corev1.Container) {
	switch cri.Service {
	case ytv1.CRIServiceNone:
		return
	case ytv1.CRIServiceCRIO:
		container.Env = append(container.Env, cri.GetCRIOEnv()...)
	case ytv1.CRIServiceContainerd:
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

	if cri.Service == ytv1.CRIServiceContainerd {
		configPath := path.Join(consts.ContainerdConfigMountPoint, consts.ContainerdConfigFileName)
		container.Args = []string{"--config", configPath}
	}

	n.addCRIServiceConfig(cri, &container)

	n.server.addCABundleMount(&container)
	n.server.addTlsSecretMount(&container)

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
		// Without dedicated job resources enforce same limits as for node.
		container.Resources.Limits = n.spec.Resources.Limits
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
